package main

import(
	
	"net/http"
	"log"
	"time"

	"go.uber.org/zap"
	"github.com/vmistry-repo/ptoclient"
)

const (
	// All times are in Milliseconds

	DNS_TIMEOUT 		= 10
	TLS_TIMEOUT 		= 10
	REQ_TIMEOUT 		= 10

	TIMEOUT_INC_INTERVAL 	= 10	// Delta by which to increase above timeouts
	MAX_TIMEOUT		= 10000	// Maximum timeout value

	// Consecutive Timeout Configs
	ENABLE_CONS_TOUT	= false
	TIMEOUT_REPORT_COUNT	= 5 	// Consecutive Timeout Report Count, after which to increase timeouts
	
	// Sliding Window Configs
	ENABLE_SLIDING_WIN	= true
	SLIDING_WIN_SIZE	= 5	// Max Size of Window to hold the last x Req status
	SLIDING_WIN_THRESHOLD 	= 50	// % of timeouts if encountered in WIN start timeout updates
)

func main() {
	logger, err := zap.NewDevelopment() // NewProduction()
	if err != nil {
	    log.Fatal(err)
	}
	logger.Level()
    
	sugar := logger.Sugar()
	sugar.Level()
	client := &http.Client{
		Timeout: time.Duration(1000) * time.Millisecond,
	}

	pclient, err := ptoclient.NewHTTPClient(client,
						ptoclient.WithDefaultTimeout(REQ_TIMEOUT),
						ptoclient.WithMaxDefaultTimeout(MAX_TIMEOUT),
						ptoclient.WithTimeoutIncInterval(TIMEOUT_INC_INTERVAL),
						ptoclient.WithTimeoutReportCount(TIMEOUT_REPORT_COUNT),
						ptoclient.WithConsTimeoutEnabled(ENABLE_CONS_TOUT),
						ptoclient.WithSlidingWindowEnabled(ENABLE_SLIDING_WIN),
						ptoclient.WithSlidingWindowSize(SLIDING_WIN_SIZE),
						ptoclient.WithSlidingWindowThreshold(SLIDING_WIN_THRESHOLD),
						ptoclient.WithCustomTransport(DNS_TIMEOUT, TLS_TIMEOUT),
						ptoclient.WithZaplogger(sugar))
	if err != nil {
		sugar.Fatalf("Client creation error: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://www.google.com", nil)
	if err != nil {
		sugar.Fatalf("Req Creation Failed: %v", err)
	}
	for i:= 0; i < 100; i++ {
		sugar.Debugf("---REQ: [%v] STARED---", i+1)
		resp, err := pclient.Do(req)
		if err != nil {
			sugar.Infof("Resp Failed: %v", err)
		} else {
			sugar.Infof("Response: [%v]", resp.Status)
		}
		sugar.Debugf("---REQ: [%v] ENDED---", i+1)
		time.Sleep(1 * time.Second)
	}
}
