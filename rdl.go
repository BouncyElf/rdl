package rdl

import (
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

type RedisClient interface {
	// get k from redis, v is the value of k,
	// ok specific whether the operation successed.
	GetString(k string) (v string, ok bool)

	// set k, v with expire time into redis,
	// ok specific whether the operation successed.
	SetString(k, v string, ex time.Duration) (ok bool)
}

type Rdl struct {
	mu  *sync.Mutex
	c   *Conf
	cli RedisClient

	set time.Time

	hasLock bool

	name string
	val  string
}

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

func (r *Rdl) Lock() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i <= r.c.retry; i++ {
		var (
			start  = time.Now()
			ticker = time.NewTicker(r.c.timeout)
			loop   = true
		)
		for loop {
			select {
			case <-ticker.C:
				loop = false
				break
			default:
				if r.getLock() {
					r.set = time.Now()
					r.hasLock = true
					time.AfterFunc(r.Remain(), func() {
						r.hasLock = false
					})
					return true
				}
				d, remain := r.c.wait, r.c.timeout-time.Now().Sub(start)
				if d > remain {
					d = remain
				}
				time.Sleep(d)
			}
		}
	}
	return false
}

func (r *Rdl) Unlock() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.hasLock {
		return
	}

	r.hasLock = false

	for !r.putLock() {
		select {
		case <-r.C():
			return
		default:
		}
	}
}

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

func (r *Rdl) Remain() time.Duration {
	if !r.hasLock {
		return 0
	}
	return r.c.timeout - time.Now().Sub(r.set)
}

func (r *Rdl) C() <-chan time.Time {
	c, remain := make(chan time.Time, 1), r.Remain()
	if int64(remain) <= 0 {
		c <- time.Now()
		return c
	}
	close(c)
	return time.NewTicker(remain).C
}

func (r *Rdl) getLock() bool {
	val, ok := r.cli.GetString(r.name)
	if !ok || val != "" {
		return false
	}
	r.val = random()
	return r.cli.SetString(r.name, val, r.c.timeout)
}

func (r *Rdl) putLock() bool {
	val, ok := r.cli.GetString(r.name)
	if ok && val == r.val {
		return r.cli.SetString(r.name, "", 1*time.Second)
	}
	return false
}

func random() string {
	return strconv.Itoa(
		rand.New(
			rand.NewSource(
				time.Now().UnixNano(),
			),
		).Int(),
	)
}

func e(msg string) error {
	return fmt.Errorf("[%s] %s.", "RDL", msg)
}
