package rdl

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

var mu *sync.Mutex

var ErrNotGetLock = e("failed to get lock")
var ErrTimeout = e("timeout")

func init() {
	mu = new(sync.Mutex)
}

// RedisClient is the interface of redis client
type RedisClient interface {
	// set k, v with expire time if origin value is origin,
	// if origin is "", this means key not exists or origin is "".
	// NOTE:
	// use (redis offical reference's script)[https://redis.io/topics/distlock]
	// to make sure this is an atomic operation.
	SetIfValIs(k, newVal string, ex time.Duration, origin string) (ok bool)
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
	mu.Lock()
	defer mu.Unlock()

	conf := DefaultConf()
	if len(confs) != 0 {
		conf = confs[0]
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
					time.AfterFunc(r.Remain(), func() {
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
					time.AfterFunc(r.Remain(), func() {
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
	defer r.mu.Unlock()

	if !r.hasLock {
		return
	}

	r.hasLock = false
	done := make(chan struct{})

	for {
		select {
		case <-r.C():
			return
		case <-time.After(10 * time.Nanosecond):
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

// Ensure make sure the func f executed, it returns only two error:
// 1.ErrNotGetLock. In this case, you can retry or whatever.
// 2.ErrTimeout. In this case, you may need to RollBack or whatever.
func (r *Rdl) Ensure(f func()) error {
	if !r.Lock() {
		return ErrNotGetLock
	}
	defer r.Unlock()

	done := make(chan struct{}, 1)
	go func() {
		f()
		done <- struct{}{}
		close(done)
	}()

	select {
	case <-time.After(r.Remain()):
		return ErrTimeout
	case <-done:
		return nil
	}
	return e("u can not get me")
}

// Remain returns the rest time of holding lock
func (r *Rdl) Remain() time.Duration {
	if !r.hasLock {
		return 0
	}
	return r.c.timeout - time.Now().Sub(r.set)
}

// C returns a stop chan, you can use this in select statement
func (r *Rdl) C() <-chan time.Time {
	c, remain := make(chan time.Time, 1), r.Remain()
	if int64(remain) <= 0 {
		c <- time.Now()
		return c
	}
	close(c)
	return time.NewTicker(remain).C
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
	return strconv.Itoa(
		rand.New(
			rand.NewSource(
				time.Now().UnixNano(),
			),
		).Int(),
	)
}

// e wrap the msg into an error
func e(msg string) error {
	return fmt.Errorf("[%s] %s.", "RDL", msg)
}
