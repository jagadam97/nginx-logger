package models

type LogEntry struct {
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
