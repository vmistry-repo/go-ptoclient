package ptoclient

import (
	"fmt"
	"time"

	"github.com/tcnksm/go-httpstat"
)

func (h *HTTPClient) consecutiveTimeoutCheck(err error,
					     result httpstat.Result,
					     url string) *HTTPStats {
	var sdata string
	var stats = &HTTPStats{}

	timedout := IsConnTimeoutErr(err)
	if timedout {
		h.log.Debugf("Timeout occured")
		h.tbitMap &= ^(1 << h.tslot)

		/*
		 * To check the if h.timeoutCount > h.maxTimeoutReport
		 * h.timeoutCount = 0
		 * h.Client.Timeout = h.Client.Timeout + h.timeoutIncInterval
		 * Here we are just updating the overall Client Timeouts but not the internal timeouts,
		 * here we will signal to calculate the internal timeouts too -- based on sliding window based counting
		 * for last N reprotings, sliding window to be configrable and default to 5
		 */
	} else {
		h.tbitMap |= (1 << h.tslot)
		sdata = fmt.Sprintf("%v", result)
		stats = getStats(sdata, url,
			int(h.Client.Timeout/time.Millisecond),
			int(h.dnstimeout/time.Millisecond), 
			int(h.tlstimeout/time.Millisecond))
	}

	h.tslot = (h.tslot + 1) % uint8(h.maxTimeoutReportCount)

	
	stats.TimeoutReason = getTimeoutErrorCause(err)
	if stats.TimeoutReason == INVALID {
		/*
		 * Print only for successful req
		 */
		 h.log.Infof("%+v", *stats)
	}
	
	if (h.tbitMap & h.tmask) == 0 {
		/*
		 * Consecutive Timeouts have occured
		 */
		if !h.enableConsTimeouts {
			return stats
		}
		h.log.Infof("%v Consecutive timeout Occured", h.maxTimeoutReportCount)
		total, dns, tls, reason := h.getTimeIncInterval(stats)
		h.updateTimeouts(total, dns, tls, reason)
	}

	return stats
}
