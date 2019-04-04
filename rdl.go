package rdl

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

var ErrNotGetLock = e("failed to get lock")
var ErrTimeout = e("timeout")
var ErrConfNotValid = e("conf is not valid")

// RedisClient is the interface of redis client
type RedisClient interface {
	// set k, v with expire time if origin value is old or not exists(nil).
	// NOTE:
	// use (redis offical reference's script)[https://redis.io/topics/distlock]
	// to make sure this is an atomic operation.
	SetIfValIs(key, new string, ex time.Duration, old string) (ok bool)
}

// Rdl is a locker
type Rdl struct {
	mu  *sync.Mutex
	c   *Conf
	cli RedisClient

	set time.Time

	hasLock bool

	name string
	val  string
}

// New returns a new lock with redisclient and key name
func New(cli RedisClient, name string, confs ...*Conf) *Rdl {
	conf := DefaultConf()
	if len(confs) != 0 {
		conf = confs[0]
	}
	if !conf.isValid() {
		panic(ErrConfNotValid)
	}
	return &Rdl{
		mu:   new(sync.Mutex),
		c:    conf,
		cli:  cli,
		name: name,
	}
}

// Lock returns whether get lock
func (r *Rdl) Lock() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i <= r.c.retry; i++ {
		var (
			start  = time.Now()
			ticker = time.NewTicker(r.c.timeout)
			loop   = true
			d      = time.Duration(0)
		)
		for loop {
			select {
			case <-ticker.C:
				loop = false
				break
			case <-time.After(d):
				if r.getLock() {
					r.set = time.Now()
					r.hasLock = true
					time.AfterFunc(r.remain(), func() {
						r.hasLock = false
					})
					return true
				}
				d = r.c.wait
				remain := r.c.timeout -
					time.Now().Sub(start)
				if d > remain {
					d = remain
				}
			}
		}
	}
	return false
}

// LockWithCancel likes `Lock` but with a context.
func (r *Rdl) LockWithContext(ctx context.Context) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i <= r.c.retry; i++ {
		var (
			start  = time.Now()
			ticker = time.NewTicker(r.c.timeout)
			loop   = true
			d      = time.Duration(0)
		)
		for loop {
			select {
			case <-ticker.C:
				loop = false
				break
			case <-ctx.Done():
				return false
			case <-time.After(d):
				if r.getLock() {
					r.set = time.Now()
					r.hasLock = true
					time.AfterFunc(r.remain(), func() {
						r.hasLock = false
					})
					return true
				}
				d = r.c.wait
				remain := r.c.timeout -
					time.Now().Sub(start)
				if d > remain {
					d = remain
				}
			}
		}
	}
	return false
}

// Unlock release the lock. If can not put lock back, Unlock wait until the
// lock expired.
func (r *Rdl) Unlock() {
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		r.hasLock = false
	}()

	if !r.hasLock {
		return
	}

	if r.putLock() {
		return
	}

	done := make(chan struct{})
	for {
		select {
		case <-time.After(r.remain()):
			return
		case <-time.After(1 * time.Nanosecond):
			go func() {
				if r.putLock() {
					done <- struct{}{}
				}
			}()
		case <-done:
			return
		}
	}
}

// Remain returns the rest time of holding lock
func (r *Rdl) remain() time.Duration {
	if !r.hasLock {
		return 0
	}
	return r.c.timeout - time.Now().Sub(r.set)
}

func (r *Rdl) renewal() {
	// TODO: if the lock can not get, it means something wrong with redis or
	// the network or the method SetIfValIs. How to fix?
	r.cli.SetIfValIs(r.name, r.val, r.c.timeout, r.val)
}

func (r *Rdl) autoRenewal() {
	for r.hasLock {
		time.AfterFunc(r.c.renewalTime, func() { r.renewal() })
	}
}

// getLock get the lock
func (r *Rdl) getLock() bool {
	v := random()
	ok := r.cli.SetIfValIs(r.name, v, r.c.timeout, "")
	if ok {
		r.val = v
	}
	return ok
}

// put lock put the lock back
func (r *Rdl) putLock() bool {
	return r.cli.SetIfValIs(r.name, "", 1*time.Second, r.val)
}

// randome returns a random string value based on time.Now().UnixNano()
func random() string {
	return getLocalIp() + strconv.Itoa(
		rand.New(
			rand.NewSource(
				time.Now().UnixNano(),
			),
		).Int(),
	)
}

// getLocalIp returns local ip
func getLocalIp() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.To4() != nil && !ip.IsLoopback() {
				return ip.To4().String()
			}
		}
	}
	return ""
}

// e wrap the msg into an error
func e(msg string) error {
	return fmt.Errorf("[%s] %s.", "RDL", msg)
}
