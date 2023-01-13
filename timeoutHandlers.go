package ptoclient

import (
	"net"
	"net/http"
	"time"
	"strings"
	"strconv"
)



 const (
	DNSLookup       = "DNSLookup"
	TCPConnection   = "TCPConnection"
	TLSHandshake    = "TLSHandshake"
	ServerProcessing= "ServerProcessing"
	ContentTransfer = "ContentTransfer"
	Total           = "Total"
)

/*
 * All values are in milliseconds
 */

type HTTPStats struct {
	DNSLookup               int     `json:"DNSLookup"`
	TCPConnection           int     `json:"TCPConnection"`
	TLSHandshake            int     `json:"TLSHandshake"`
	ServerProcessing        int     `json:"ServerProcessing"`
	ContentTransfer         int     `json:"ContentTransfer"`
	Total                   int     `json:"Total"`
	URL                     string  `json:"URL"`
	ReqTimeout              int     `json:"ClientTimeout"`
	DnsTimeout 		int 	`json:"DNSTimeout"`
	TlsTimeout 		int 	`json:"TLSTimeout"`
	TimeoutReason		int 	`json:"TimeoutReason"`
	Message                 string  `json:"message"`
}

const (
	DNS_TIMEOUT = "dial tcp"
	TLS_TIMEOUT = "TLS handshake timeout"
	REQ_TIMEOUT = "Client.Timeout exceeded while awaiting headers"
)

const (
	INVALID = -1
	DNS_TIMEOUT_I = 1
	TLS_TIMEOUT_I = 2
	REQ_TIMEOUT_I = 4
)

var TIMEOUT_CAUSE = map[int]string {
	DNS_TIMEOUT_I: DNS_TIMEOUT,
	TLS_TIMEOUT_I: TLS_TIMEOUT,
	REQ_TIMEOUT_I: REQ_TIMEOUT,
}

func (h *HTTPClient) getTransportWithDNSTimeout(dnstimeoutIncInterval time.Duration,
						tlstimeoutIncInterval time.Duration,
						) http.RoundTripper {
	dialer := (&net.Dialer{
		Timeout: dnstimeoutIncInterval,
		KeepAlive: 2,
		DualStack: true,
		}).DialContext
	
	return &http.Transport {
		DialContext: dialer,
		MaxIdleConns: 64,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout: 10,
		TLSHandshakeTimeout: tlstimeoutIncInterval,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (h *HTTPClient) getTransportWithtTLSTimeout(tlstimeoutIncInterval time.Duration,
						) http.RoundTripper {
	return &http.Transport {
		DialContext: h.Dialer,
		MaxIdleConns: 64,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout: 10,
		TLSHandshakeTimeout: tlstimeoutIncInterval,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (h *HTTPClient) updateTimeouts(timeoutIncInterval time.Duration,
				    dnstimeoutIncInterval time.Duration,
				    tlstimeoutIncInterval time.Duration,
				    reason int) {
	var tport http.RoundTripper

	if reason & DNS_TIMEOUT_I == DNS_TIMEOUT_I {
		h.log.Infof("Updating DNS timeouts from %v to %v", h.dnstimeout,
			   dnstimeoutIncInterval)
		if (dnstimeoutIncInterval <= time.Duration(h.maxClientTimeout)) {
			tport = h.getTransportWithDNSTimeout(dnstimeoutIncInterval,
				h.tlstimeout)
			h.Client.Transport = tport
		} else {
			h.log.Errorf("Can't Update DNS Timeout,"+
			             "Reached to its maximum %v", h.maxClientTimeout)
			return
		}
		h.dnstimeout = dnstimeoutIncInterval
		h.Client.Transport = tport
		return
	}else if reason & TLS_TIMEOUT_I == TLS_TIMEOUT_I {
		h.log.Infof("Updating TLS timeouts from %v to %v", h.tlstimeout,
			   tlstimeoutIncInterval)
		if (tlstimeoutIncInterval <= time.Duration(h.maxClientTimeout)) {
			tport = h.getTransportWithtTLSTimeout(tlstimeoutIncInterval)
			h.Client.Transport = tport
		} else {
			h.log.Errorf("Can't Update TLS Timeout,"+
			             "Reached to its maximum %v", h.maxClientTimeout)
			return
		}
		h.tlstimeout = tlstimeoutIncInterval
		h.Client.Transport = tport
		return
	}

	if (timeoutIncInterval <= time.Duration(h.maxClientTimeout)) {
		h.log.Infof("Updating REQ timeouts from %v to %v", h.Client.Timeout,
			    timeoutIncInterval)
		h.Client.Timeout = timeoutIncInterval
	} else {
		h.log.Errorf("Can't Update REQ Timeout,"+
			     "Reached to its maximum %v", h.maxClientTimeout)
		/*
		 * Function Handler to call if this situation occurs,
		 * Might want to do some operations
		 */
	}
	return
}

func IsConnTimeoutErr(err error) bool {
	var ok		bool
	var netErr	net.Error

	if err == nil {
		return false
	}
	if netErr, ok = err.(net.Error); ok {
		return netErr.Timeout()
	}	
	return false
}

func getTimeoutErrorCause(err error) int {
	if err == nil {
		return INVALID
	}
	if strings.Contains(err.Error(), DNS_TIMEOUT) {
		return DNS_TIMEOUT_I
	}
	if strings.Contains(err.Error(), TLS_TIMEOUT) {
		return TLS_TIMEOUT_I
	}
	if strings.Contains(err.Error(), REQ_TIMEOUT) {
		return REQ_TIMEOUT_I
	}
	return INVALID
}

func getStats(str, url string, clientTimeout, dnsTimeout, tlsTimeout int) *HTTPStats {
	var m map[string]int = make(map[string]int)

	data := strings.Split(str, ",")

	for _, val := range data {
		fields := strings.Split(val, ": ")
		fields[0] = strings.TrimSpace(fields[0])
		fields[1] = strings.Split(fields[1], " ms")[0]
		val, err := strconv.Atoi(fields[1])
		if err != nil {
			val = INVALID
		}
		m[fields[0]] = val
	}

	return logHttpStats(m, url, clientTimeout, dnsTimeout, tlsTimeout)
}

func logHttpStats(stats map[string]int, url string,
		  timeout, dnsTimeout, tlsTimeout int) *HTTPStats {

	var httpstats *HTTPStats = &HTTPStats {
	        DNSLookup: stats[DNSLookup],
	        TCPConnection: stats[TCPConnection],
	        TLSHandshake: stats[TLSHandshake],
	        ServerProcessing: stats[ServerProcessing],
	        ContentTransfer: stats[ContentTransfer],
	        Total: stats[Total],
	        URL: url,
	        ReqTimeout: timeout,
		DnsTimeout: dnsTimeout,
		TlsTimeout: tlsTimeout,
	        Message: "HTTP Stats",
	}
	return httpstats
}

func getNoTimeOutVal(v int) uint8 {
	var maxMissVal	uint8 = 0
	var hbMaxMiss	uint8 = uint8(v)
	var idx		uint8

	for idx = 0; idx < hbMaxMiss; idx++ {
		maxMissVal |= (1 << idx)
	}
	return maxMissVal
}