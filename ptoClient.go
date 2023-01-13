package ptoclient 

import (
	"log"
	"net"
	"net/http"
	"time"
	"context"

	"go.uber.org/zap"
	"github.com/tcnksm/go-httpstat"
)

type Option func(*HTTPClient) error

const (
	ClientDefaultTimeout		= time.Duration(1000) * time.Millisecond // Sec
	ClientTimeoutIncInterval	= time.Duration(2000)  * time.Millisecond // Sec
	ClientTimeoutReportCount	= 5  // Count
	ClientWindowDefaultThreshold	= 70 // indicates % of timeout reports when window is full
	ClientWindowDefaultSize		= 5 // Window Size
)

type HTTPClient struct {
	*http.Client

	maxTimeoutReportCount	int // After this many report increment Client Timeout
	maxClientTimeout	time.Duration // Max value till increment of timeout
	timeoutIncInterval	time.Duration // By which val, timeout to increment

	tslot 			uint8
	tbitMap			uint8
	tmask			uint8

	window 			Window // Sliding Window

	enableConsTimeouts	bool
	enableSlidingWinTouts	bool

	Dialer 			func(ctx context.Context, network string, address string) (net.Conn, error)
	dnstimeout 		time.Duration
	tlstimeout 		time.Duration
	log 			*zap.SugaredLogger
}


func NewHTTPClient(h *http.Client, opts ...Option) (*HTTPClient, error) {
	logger, err := zap.NewProduction() //NewDevelopment() // NewProduction()
	if err != nil {
	    log.Fatal(err)
	}
    
	sugar := logger.Sugar()
    
	client := &HTTPClient{
		Client: h,
		maxClientTimeout: ClientDefaultTimeout,
		maxTimeoutReportCount: ClientTimeoutReportCount,
		timeoutIncInterval: ClientTimeoutIncInterval,

		tslot: 0,
		tbitMap: getNoTimeOutVal(ClientTimeoutReportCount),
		tmask: getNoTimeOutVal(ClientTimeoutReportCount),
		
		window: Window{
			size: ClientWindowDefaultSize,
			threshold: ClientWindowDefaultThreshold,
			log: sugar,
		},

		enableConsTimeouts: false,
		enableSlidingWinTouts: false,
		log: sugar,
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

func WithDefaultTimeout(val int) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.Client.Timeout = time.Duration(val) * time.Millisecond
		return nil
	}
}

func WithCustomTransport(dialtimeout, tlstimeout int) func(*HTTPClient) error {
	dialer := (&net.Dialer{
		Timeout: time.Duration(dialtimeout) * time.Millisecond,
		KeepAlive: 2,
		DualStack: true,
		}).DialContext
	
	transport := &http.Transport {
		DialContext: dialer,
		MaxIdleConns: 64,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout: 10,
		TLSHandshakeTimeout: time.Duration(tlstimeout) * time.Millisecond,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return func(httpClient *HTTPClient) error {
		httpClient.dnstimeout = time.Duration(dialtimeout) * time.Millisecond
		httpClient.tlstimeout = time.Duration(tlstimeout) * time.Millisecond
		httpClient.Dialer = dialer
		httpClient.Client.Transport = transport
		return nil
	}
}

func WithMaxDefaultTimeout(val int) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.maxClientTimeout = time.Duration(val) * time.Millisecond
		return nil
	}
}

func WithTimeoutIncInterval(val int) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.timeoutIncInterval = time.Duration(val) * time.Millisecond
		return nil
	}
}

func WithTimeoutReportCount(val int) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.maxTimeoutReportCount = val
		httpClient.tslot = 0
		httpClient.tbitMap = getNoTimeOutVal(val)
		httpClient.tmask = getNoTimeOutVal(val)
		return nil
	}
}

/*
func WithCustomClient(c *http.Client) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.Client = c
		return nil
	}
}
*/

func WithSlidingWindowSize(val int) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.window.size = val
		return nil
	}
}

func WithSlidingWindowThreshold(val int) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.window.threshold = val
		return nil
	}
}

func WithConsTimeoutEnabled(val bool) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.enableConsTimeouts = val
		return nil
	}
}

func WithSlidingWindowEnabled(val bool) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.enableSlidingWinTouts = val
		return nil
	}
}

func WithZaplogger(log *zap.SugaredLogger) func(*HTTPClient) error {
	return func(httpClient *HTTPClient) error {
		httpClient.log = log
		httpClient.window.log = log
		return nil
	}
}

func (h *HTTPClient) Do (req *http.Request) (*http.Response, error) {
	var result  	httpstat.Result
	var ctx 	context.Context
	var stats       *HTTPStats = nil

	ctx = httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	resp, err :=  h.Client.Do(req)
	result.End(time.Now())

	stats = h.consecutiveTimeoutCheck(err, result, req.URL.String())
	
	h.NotifySlidingWindow(stats)

	return resp, err
}