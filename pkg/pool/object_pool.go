package pool

import (
	"bytes"
	"sync"

	"high-go-press/internal/biz"
)

var (
	// 响应对象池 - 复用API响应对象
	responsePool = sync.Pool{
		New: func() interface{} {
			return &biz.CounterResponse{}
		},
	}

	// 请求对象池 - 复用API请求对象
	requestPool = sync.Pool{
		New: func() interface{} {
			return &biz.IncrementRequest{}
		},
	}

	// 字节缓冲池 - 复用字节缓冲区
	bufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	// 字符串切片池 - 复用字符串切片
	stringSlicePool = sync.Pool{
		New: func() interface{} {
			slice := make([]string, 0, 10) // 预分配容量
			return &slice
		},
	}
)

// ObjectPool 对象池管理器
type ObjectPool struct {
	// 统计信息
	responseGets    int64
	responsePuts    int64
	requestGets     int64
	requestPuts     int64
	bufferGets      int64
	bufferPuts      int64
	stringSliceGets int64
	stringSlicePuts int64

	mu sync.RWMutex
}

// NewObjectPool 创建对象池管理器
func NewObjectPool() *ObjectPool {
	return &ObjectPool{}
}

// GetCounterResponse 从池中获取响应对象
func (p *ObjectPool) GetCounterResponse() *biz.CounterResponse {
	p.mu.Lock()
	p.responseGets++
	p.mu.Unlock()

	resp := responsePool.Get().(*biz.CounterResponse)
	// 重置对象状态
	resp.ResourceID = ""
	resp.CounterType = ""
	resp.CurrentValue = 0
	resp.Success = false
	resp.Message = ""
	resp.Timestamp = 0

	return resp
}

// PutCounterResponse 将响应对象归还到池中
func (p *ObjectPool) PutCounterResponse(resp *biz.CounterResponse) {
	if resp == nil {
		return
	}

	p.mu.Lock()
	p.responsePuts++
	p.mu.Unlock()

	responsePool.Put(resp)
}

// GetIncrementRequest 从池中获取请求对象
func (p *ObjectPool) GetIncrementRequest() *biz.IncrementRequest {
	p.mu.Lock()
	p.requestGets++
	p.mu.Unlock()

	req := requestPool.Get().(*biz.IncrementRequest)
	// 重置对象状态
	req.ResourceID = ""
	req.CounterType = ""
	req.Delta = 0

	return req
}

// PutIncrementRequest 将请求对象归还到池中
func (p *ObjectPool) PutIncrementRequest(req *biz.IncrementRequest) {
	if req == nil {
		return
	}

	p.mu.Lock()
	p.requestPuts++
	p.mu.Unlock()

	requestPool.Put(req)
}

// GetBuffer 从池中获取字节缓冲区
func (p *ObjectPool) GetBuffer() *bytes.Buffer {
	p.mu.Lock()
	p.bufferGets++
	p.mu.Unlock()

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset() // 清空缓冲区
	return buf
}

// PutBuffer 将字节缓冲区归还到池中
func (p *ObjectPool) PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	p.mu.Lock()
	p.bufferPuts++
	p.mu.Unlock()

	// 防止缓冲区过大占用内存
	if buf.Cap() > 64*1024 { // 64KB
		return
	}

	bufferPool.Put(buf)
}

// GetStringSlice 从池中获取字符串切片
func (p *ObjectPool) GetStringSlice() *[]string {
	p.mu.Lock()
	p.stringSliceGets++
	p.mu.Unlock()

	slice := stringSlicePool.Get().(*[]string)
	*slice = (*slice)[:0] // 重置长度但保留容量
	return slice
}

// PutStringSlice 将字符串切片归还到池中
func (p *ObjectPool) PutStringSlice(slice *[]string) {
	if slice == nil {
		return
	}

	p.mu.Lock()
	p.stringSlicePuts++
	p.mu.Unlock()

	// 防止切片过大占用内存
	if cap(*slice) > 100 {
		return
	}

	stringSlicePool.Put(slice)
}

// GetStats 获取对象池统计信息
func (p *ObjectPool) GetStats() ObjectPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ObjectPoolStats{
		Response: PoolUsage{
			Gets: p.responseGets,
			Puts: p.responsePuts,
			Hit:  calculateHitRate(p.responseGets, p.responsePuts),
		},
		Request: PoolUsage{
			Gets: p.requestGets,
			Puts: p.requestPuts,
			Hit:  calculateHitRate(p.requestGets, p.requestPuts),
		},
		Buffer: PoolUsage{
			Gets: p.bufferGets,
			Puts: p.bufferPuts,
			Hit:  calculateHitRate(p.bufferGets, p.bufferPuts),
		},
		StringSlice: PoolUsage{
			Gets: p.stringSliceGets,
			Puts: p.stringSlicePuts,
			Hit:  calculateHitRate(p.stringSliceGets, p.stringSlicePuts),
		},
	}
}

// ObjectPoolStats 对象池统计信息
type ObjectPoolStats struct {
	Response    PoolUsage `json:"response"`
	Request     PoolUsage `json:"request"`
	Buffer      PoolUsage `json:"buffer"`
	StringSlice PoolUsage `json:"string_slice"`
}

// PoolUsage 池使用情况
type PoolUsage struct {
	Gets int64   `json:"gets"`
	Puts int64   `json:"puts"`
	Hit  float64 `json:"hit_rate"` // 命中率
}

// calculateHitRate 计算命中率
func calculateHitRate(gets, puts int64) float64 {
	if gets == 0 {
		return 0
	}
	return float64(puts) / float64(gets) * 100
}
