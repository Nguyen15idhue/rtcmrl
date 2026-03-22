package worker

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/your-org/rtcm-stream-gateway/internal/engine"
	"github.com/your-org/rtcm-stream-gateway/internal/metrics"
)

type Pool struct {
	cfg         PoolConfig
	eng         *engine.Engine
	inCh        chan engine.InFrame
	dispatchChs []chan engine.InFrame
	wg          sync.WaitGroup
	mu          sync.RWMutex
	active      int32
	desired     int32
	ctx         context.Context
	cancel      context.CancelFunc

	scaleUpThresh   float64
	scaleDownThresh float64
	scaleInterval   time.Duration
	autoScale       bool
}

type PoolConfig struct {
	Min, Max        int
	QueueSize       int
	AutoScale       bool
	ScaleUpThresh   float64
	ScaleDownThresh float64
	ScaleInterval   time.Duration
}

func NewPool(ctx context.Context, cfg PoolConfig, eng *engine.Engine) *Pool {
	if cfg.Min <= 0 {
		cfg.Min = 4
	}
	if cfg.Max < cfg.Min {
		cfg.Max = cfg.Min
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 4096
	}
	if cfg.ScaleUpThresh <= 0 {
		cfg.ScaleUpThresh = 0.8
	}
	if cfg.ScaleDownThresh <= 0 {
		cfg.ScaleDownThresh = 0.3
	}
	if cfg.ScaleInterval <= 0 {
		cfg.ScaleInterval = 10 * time.Second
	}

	pctx, cancel := context.WithCancel(ctx)
	p := &Pool{
		cfg:             cfg,
		eng:             eng,
		scaleUpThresh:   cfg.ScaleUpThresh,
		scaleDownThresh: cfg.ScaleDownThresh,
		scaleInterval:   cfg.ScaleInterval,
		autoScale:       cfg.AutoScale,
		ctx:             pctx,
		cancel:          cancel,
	}
	p.inCh = make(chan engine.InFrame, cfg.QueueSize)
	metrics.QueueCapacity.Set(float64(cfg.QueueSize))

	atomic.StoreInt32(&p.desired, int32(cfg.Min))
	p.scaleToLocked(int32(cfg.Min))

	if p.autoScale {
		go p.autoScaler()
	}

	return p
}

func (p *Pool) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-p.inCh:
			metrics.QueueDepth.Set(float64(len(p.inCh)))
			p.dispatch(f)
		}
	}
}

func (p *Pool) Input(f engine.InFrame) {
	select {
	case p.inCh <- f:
		metrics.QueueDepth.Set(float64(len(p.inCh)))
	default:
		metrics.FramesDropped.Inc()
	}
}

func (p *Pool) dispatch(f engine.InFrame) {
	idx := int(f.Frame[0]^f.Frame[1]^f.Frame[2]) % len(p.dispatchChs)
	select {
	case p.dispatchChs[idx] <- f:
	default:
		metrics.FramesDropped.Inc()
	}
}

func (p *Pool) scaleToLocked(n int32) {
	if n < int32(p.cfg.Min) {
		n = int32(p.cfg.Min)
	}
	if n > int32(p.cfg.Max) {
		n = int32(p.cfg.Max)
	}

	cur := int32(len(p.dispatchChs))
	if cur == n {
		return
	}

	if n > cur {
		for i := int32(cur); i < n; i++ {
			ch := make(chan engine.InFrame, p.cfg.QueueSize/p.cfg.Max)
			if p.cfg.QueueSize/p.cfg.Max == 0 {
				ch = make(chan engine.InFrame, 1)
			}
			p.dispatchChs = append(p.dispatchChs, ch)
			p.wg.Add(1)
			go p.worker(i, ch)
		}
	} else {
		for i := cur - 1; i >= n; i-- {
			close(p.dispatchChs[i])
			p.dispatchChs = p.dispatchChs[:i]
		}
	}
}

func (p *Pool) worker(id int32, ch chan engine.InFrame) {
	defer p.wg.Done()
	atomic.AddInt32(&p.active, 1)
	metrics.ActiveWorkers.Set(float64(atomic.LoadInt32(&p.active)))
	defer func() {
		atomic.AddInt32(&p.active, -1)
		metrics.ActiveWorkers.Set(float64(atomic.LoadInt32(&p.active)))
	}()

	for f := range ch {
		p.eng.Input(f)
	}
}

func (p *Pool) SetDesiredWorkers(n int) {
	atomic.StoreInt32(&p.desired, int32(n))
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scaleToLocked(int32(n))
}

func (p *Pool) DesiredWorkers() int {
	return int(atomic.LoadInt32(&p.desired))
}

func (p *Pool) ActiveWorkers() int {
	return int(atomic.LoadInt32(&p.active))
}

func (p *Pool) QueueSize() int {
	return len(p.inCh)
}

func (p *Pool) SetAutoScale(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.autoScale = enabled
}

func (p *Pool) autoScaler() {
	ticker := time.NewTicker(p.scaleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.evaluateScale()
		}
	}
}

func (p *Pool) evaluateScale() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.autoScale {
		return
	}

	queueFill := float64(len(p.inCh)) / float64(p.cfg.QueueSize)
	cpuFill := float64(runtime.NumGoroutine()) / 1000.0

	load := queueFill
	if cpuFill > load {
		load = cpuFill
	}

	desired := atomic.LoadInt32(&p.desired)
	max := int32(p.cfg.Max)
	min := int32(p.cfg.Min)

	if load >= p.scaleUpThresh && desired < max {
		newDes := desired + 1
		if newDes > max {
			newDes = max
		}
		atomic.StoreInt32(&p.desired, newDes)
		p.scaleToLocked(newDes)
		return
	}

	if load <= p.scaleDownThresh && desired > min {
		newDes := desired - 1
		if newDes < min {
			newDes = min
		}
		atomic.StoreInt32(&p.desired, newDes)
		p.scaleToLocked(newDes)
	}
}

func (p *Pool) Stop() {
	p.cancel()
	close(p.inCh)
	for _, ch := range p.dispatchChs {
		close(ch)
	}
	p.wg.Wait()
}
