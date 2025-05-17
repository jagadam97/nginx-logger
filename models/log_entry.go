package models

import "time"

type LogEntry struct {
	TimeLocal            time.Time `json:"time_local"`
	RemoteAddr           string    `json:"remote_addr"`
	RequestURI           string    `json:"request_uri"`
	Status               uint16    `json:"status"`
	ServerName           string    `json:"server_name"`
	RequestTime          float64   `json:"request_time"`
	RequestMethod        string    `json:"request_method"`
	BytesSent            uint64    `json:"bytes_sent"`
	HTTPHost             string    `json:"http_host"`
	ServerProtocol       string    `json:"server_protocol"`
	UpstreamAddr         string    `json:"upstream_addr"`
	UpstreamResponseTime float64   `json:"upstream_response_time"`
	SSLProtocol          string    `json:"ssl_protocol"`
	SSLCipher            string    `json:"ssl_cipher"`
	HTTPUserAgent        string    `json:"http_user_agent"`
}
