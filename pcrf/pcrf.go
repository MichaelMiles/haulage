package pcrf

import (
	"database/sql"
	"encoding/binary"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

func StartPCRF(db *sql.DB) {
	handler := makeDataHandler(db)
	server := radius.PacketServer{
		Handler:      radius.HandlerFunc(handler),
		SecretSource: radius.StaticSecretSource([]byte(`secret`)),
	}

	log.Info("Starting server on: 1813")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func makeDataHandler(db *sql.DB) func(w radius.ResponseWriter, r *radius.Request) {
	return func(w radius.ResponseWriter, r *radius.Request) {
		packet := r.Packet
		user := rfc2865.UserName_GetString(packet)
		dataBytes := rfc2865.State_Get(packet)
		//localUpBytes := int64(binary.LittleEndian.Uint64(dataBytes[0:8]))
		//localDownBytes := int64(binary.LittleEndian.Uint64(dataBytes[8:16]))
		extUpBytes := int64(binary.LittleEndian.Uint64(dataBytes[16:24]))
		extDownBytes := int64(binary.LittleEndian.Uint64(dataBytes[24:32]))

		ip_string := strings.Split(user, ":")[0]
		logUsage(db, ip_string, extUpBytes, extDownBytes)
	}
}

func logUsage(db *sql.DB, user_ip string, upBytes int64, downBytes int64) {
	// Try to save to the database 3 times then give up
	for i := 0; i < 3; i++ {
		trx, err := db.Begin()
		if err != nil {
			log.WithField("UserIp", user_ip).WithError(err).Error("Unable to begin transaction")
			return
		}

		var imsi int64
		err = trx.QueryRow("select imsi from static_ips where ip=?", user_ip).Scan(&imsi)
		if err != nil {
			log.WithField("ip", user_ip).WithError(err).Error("Unable to lookup imsi's static ip address")
			// TODO Consider deferring the rollback?
			trx.Rollback()
			return
		}

		var (
			rawDown     int64
			rawUp       int64
			dataBalance int64
			balance     decimal.Decimal
			bridged     bool
			enabled     bool
		)

		err = trx.QueryRow(
			"SELECT raw_down, raw_up, data_balance, balance, bridged, enabled FROM customers WHERE imsi=? ",
			imsi).Scan(&rawDown, &rawUp, &dataBalance, &balance, &bridged, &enabled)
		if err != nil {
			log.WithField("imsi", imsi).WithError(err).Error("Unable to lookup customer data")
			trx.Rollback()
			return
		}

		rawDown += downBytes
		rawUp += upBytes
		dataBalance -= upBytes
		dataBalance -= downBytes

		if dataBalance < 0 {
			// Negative balance may occur since there is a race condition between when packets are counted
			// and when the flow is cut off with iptables.
			// For now per network policy don't allow a negative data balance. Some data may not be billed.
			log.WithField("imsi", imsi).WithField("data_balance", dataBalance).Debug(
				"Zeroing out negative data balance")
			dataBalance = 0
		}

		_, err = trx.Exec(
			"UPDATE customers SET raw_down=?, raw_up=?, data_balance=?, enabled=?, bridged=? WHERE imsi=?",
			rawDown, rawUp, dataBalance, enabled, bridged, imsi)
		if err != nil {
			log.WithField("imsi", imsi).WithError(err).Error("Unable to execute update customer data")
			trx.Rollback()
			return
		}

		err = trx.Commit()
		if err != nil {
			log.WithField("Attempt", i).WithField("imsi", imsi).WithError(err).Warn("Unable to commit")
		} else {
			return
		}
	}
	log.WithField("User", user_ip).Error("Giving up committing billing update!")
	return
}
