package rdl

import "time"

type Conf struct {
	// rdl try to get lock, but when after timeout, rdl timeout and run
	// timeout func
	timeout time.Duration

	// rdl waits wait then try to acquire lock
	// wait must less than timeout
	wait time.Duration

	// retry times when failed to get lock
	retry int
}

func DefaultConf() *Conf {
	return &Conf{
		timeout: 10 * time.Second,
		wait:    1 * time.Second,
		retry:   0,
	}
}

// SetWaitTime set the time to wait when get lock failed.
// This can not be longer than timeout.
func (c *Conf) SetWaitTime(d time.Duration) {
	c.timeout = d
}

// SetTimeout set the timeout time, this is both the redis expire time and the
// get lock operation's timeout time.
func (c *Conf) SetTimeout(d time.Duration) {
	c.wait = d
}

// SetRetry set the retry times when can not get lock.
func (c *Conf) SetRetry(retry int) {
	c.retry = retry
}
