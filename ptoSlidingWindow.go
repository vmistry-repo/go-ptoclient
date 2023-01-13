package ptoclient

import (
	"time"

	"go.uber.org/zap"
	"github.com/dariubs/percent"
)

const PERCENT = "%"
type Window struct {
	size		int
	ele 		[]*HTTPStats
	threshold 	int

	// Request Counts
	timeouts 	int

	log 		*zap.SugaredLogger
}

func (w *Window) Print(p bool) {
	if p {
		w.log.Debugf("Current Window Size: %v, Window Capacity: %v," +
		    " Window Timeout Threshold: %v," +
		    " Window Req Timeout Count: %v", len(w.ele), w.size,
		    w.threshold, w.timeouts)
	}
}

func (w *Window) isFull() bool {
	if w.size == len(w.ele) {
		return true
	}
	return false
}

func (w *Window) AddEle(stat *HTTPStats) {
	w.RemoveEle()
	if len(w.ele) < w.size {
		w.ele = append(w.ele, stat)
		if stat.TimeoutReason != INVALID {
			w.timeouts++
		}
	}
}

func (w *Window) RemoveEle() {
	if w.isFull() && w.size > 0 {
		if w.ele[0].TimeoutReason != INVALID {
			w.timeouts--
		}
		w.ele = w.ele[1:]
	}
}

/*
 * % of Timeouts in the total Sliding Window 
 */

func (w *Window) GetTimeoutThreshold() int {
	return int(percent.PercentOf(w.timeouts, w.size))
}

func (h *HTTPClient) getTimeIncInterval(stats *HTTPStats) (time.Duration, time.Duration,
				           time.Duration, int) {
	var ret time.Duration
	var dns_ret time.Duration
	var tls_ret time.Duration
	var mask int
        var data *HTTPStats = stats

	if data != nil &&
	   data.TimeoutReason != INVALID {
		mask |= data.TimeoutReason
	}
	if mask & DNS_TIMEOUT_I == DNS_TIMEOUT_I {
		dns_ret = h.dnstimeout + h.timeoutIncInterval
	}
	if mask & TLS_TIMEOUT_I == TLS_TIMEOUT_I {
		tls_ret = h.tlstimeout + h.timeoutIncInterval
	}
	if mask & REQ_TIMEOUT_I == REQ_TIMEOUT_I {
		ret = h.Timeout + h.timeoutIncInterval
	}
	if mask == 0 {
		panic(0)
	}
	return ret, dns_ret, tls_ret, mask
}

func (h *HTTPClient) NotifySlidingWindow(stats *HTTPStats) {
	h.window.AddEle(stats)
	h.window.Print(h.enableSlidingWinTouts)
	if h.enableSlidingWinTouts && !h.window.isFull() {
		h.log.Debugf("Run Sliding Window ptoclient after:" +
			     "%v rest attempts\n", h.window.size - len(h.window.ele))
	}
	if !h.enableSlidingWinTouts || !h.window.isFull() {
		return
	}
	// We have to update timeouts only if we recieved new timeouts
	if stats.TimeoutReason == INVALID {
		return
	}
	h.log.Debugf("Current Window Timeout Threshold: %v%v, Window Timeout Threshold Limit: %v%v", 
		     h.window.GetTimeoutThreshold(), PERCENT, h.window.threshold, PERCENT)
	if h.window.GetTimeoutThreshold() > h.window.threshold {
		// Update timeouts
		h.log.Debugf("Window Threshold Reached [%v%v] is > [%v%v]", 
			     h.window.GetTimeoutThreshold(), PERCENT,
			     h.window.threshold, PERCENT)
		total, dns, tls, reason := h.getTimeIncInterval(stats)
		h.updateTimeouts(total, dns, tls, reason)
	}
}