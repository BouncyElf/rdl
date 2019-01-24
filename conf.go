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

	timeoutFunc func()

	redisPrefix string
}

func DefaultConf() *Conf {
	return &Conf{
		timeout:     10 * time.Second,
		wait:        1 * time.Second,
		timeoutFunc: func() {},
	}
}

func (c *Conf) SetTimeoutFunc(f func()) {
	c.timeoutFunc = f
}

func (c *Conf) SetWaitTime(d time.Duration) {
	c.timeout = d
}

func (c *Conf) SetTimeout(d time.Duration) {
	c.wait = d
}

func (c *Conf) SetRedisPrefix(s string) {
	c.redisPrefix = s
}

func (c *Conf) SetRetry(retry int) {
	c.retry = retry
}
