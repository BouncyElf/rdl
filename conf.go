package rdl

import "time"

type Conf struct {
	// rdl try to get lock, but when after timeout, rdl timeout and run
	// timeout func
	timeout time.Duration

	// rdl waits wait then try to acquire lock
	// wait must less than timeout
	wait time.Duration

	// autoRenewal represents if auto renewal the lock every renewalTime
	autoRenewal bool

	// renewalTime must less than timeout
	renewalTime time.Duration

	// retry times when failed to get lock
	retry int
}

func DefaultConf() *Conf {
	return &Conf{
		timeout:     30 * time.Second,
		wait:        1 * time.Second,
		retry:       0,
		autoRenewal: true,
		renewalTime: 10 * time.Second,
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

// SetAutoRenewal set the autoRenewal.
func (c *Conf) SetAutoRenewal(on bool) {
	c.autoRenewal = on
}

// SetRenewalTime set the renewalTime, this option can be useless when autoRenewal
// is false.
func (c *Conf) SetRenewalTime(d time.Duration) {
	c.renewalTime = d
}

// isValid returns if the conf is valid.
func (c *Conf) isValid() bool {
	return !((c.autoRenewal && c.renewalTime < c.timeout) || c.wait > c.timeout)
}
