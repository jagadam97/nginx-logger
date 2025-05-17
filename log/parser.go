package log

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jagadam97/nginx-logger/models"
)

// rawLogEntry is used to unmarshal the JSON log line where all fields are initially strings.
type rawLogEntry struct {
	TimeLocal            string `json:"time_local"`
	RemoteAddr           string `json:"remote_addr"`
	RequestURI           string `json:"request_uri"`
	Status               string `json:"status"`
	ServerName           string `json:"server_name"`
	RequestTime          string `json:"request_time"`
	RequestMethod        string `json:"request_method"`
	BytesSent            string `json:"bytes_sent"`
	HTTPHost             string `json:"http_host"`
	ServerProtocol       string `json:"server_protocol"`
	UpstreamAddr         string `json:"upstream_addr"`
	UpstreamResponseTime string `json:"upstream_response_time"`
	SSLProtocol          string `json:"ssl_protocol"`
	SSLCipher            string `json:"ssl_cipher"`
	HTTPUserAgent        string `json:"http_user_agent"`
}

func ParseLogEntry(line string) (models.LogEntry, error) {
	var raw rawLogEntry
	err := json.Unmarshal([]byte(line), &raw)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("error unmarshaling json: %w", err)
	}

	var logEntry models.LogEntry
	const timeLayout = "02/Jan/2006:15:04:05 -0700"

	if raw.TimeLocal != "" {
		parsedTime, timeErr := time.Parse(timeLayout, raw.TimeLocal)
		if timeErr != nil {
			return models.LogEntry{}, fmt.Errorf("error parsing TimeLocal '%s': %w", raw.TimeLocal, timeErr)
		}
		logEntry.TimeLocal = parsedTime
	}

	logEntry.RemoteAddr = raw.RemoteAddr
	logEntry.RequestURI = raw.RequestURI
	logEntry.ServerName = raw.ServerName
	logEntry.RequestMethod = raw.RequestMethod
	logEntry.HTTPHost = raw.HTTPHost
	logEntry.ServerProtocol = raw.ServerProtocol
	logEntry.UpstreamAddr = raw.UpstreamAddr
	logEntry.SSLProtocol = raw.SSLProtocol
	logEntry.SSLCipher = raw.SSLCipher
	logEntry.HTTPUserAgent = raw.HTTPUserAgent

	
	if raw.Status != "" {
		status, convErr := strconv.ParseUint(raw.Status, 10, 16)
		if convErr != nil {
			return models.LogEntry{}, fmt.Errorf("error parsing Status '%s': %w", raw.Status, convErr)
		}
		logEntry.Status = uint16(status)
	}

	if raw.RequestTime != "" {
		requestTime, convErr := strconv.ParseFloat(raw.RequestTime, 64)
		if convErr != nil {
			return models.LogEntry{}, fmt.Errorf("error parsing RequestTime '%s': %w", raw.RequestTime, convErr)
		}
		logEntry.RequestTime = requestTime
	}

	if raw.BytesSent != "" {
		bytesSent, convErr := strconv.ParseUint(raw.BytesSent, 10, 64)
		if convErr != nil {
			return models.LogEntry{}, fmt.Errorf("error parsing BytesSent '%s': %w", raw.BytesSent, convErr)
		}
		logEntry.BytesSent = bytesSent
	}

	if raw.UpstreamResponseTime != "" {
		upstreamResponseTime, convErr := strconv.ParseFloat(raw.UpstreamResponseTime, 64)
		if convErr != nil {
			return models.LogEntry{}, fmt.Errorf("error parsing UpstreamResponseTime '%s': %w", raw.UpstreamResponseTime, convErr)
		}
		logEntry.UpstreamResponseTime = upstreamResponseTime
	}

	return logEntry, nil
}
