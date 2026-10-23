package auth

import (
	"net/netip"
	"sync"
	"time"
)

const loginFailureLimit = 5
const loginPeerLimit = 1024
const loginWindow = time.Minute

type loginAttempts struct {
	until            time.Time
	failures, active int
}

type loginProtection struct {
	mu    sync.Mutex
	peers map[netip.Addr]*loginAttempts
}

func newLoginProtection() *loginProtection {
	return &loginProtection{peers: make(map[netip.Addr]*loginAttempts)}
}

// begin 按已验证来源 IP 限制一分钟内五次失败；在途预留避免并发穿透。
// 成功不清空失败次数，防止用有效凭据替攻击者重置预算；键上限防止内存放大。
func (p *loginProtection) begin(peer netip.Addr, now time.Time) (func(bool), time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, entry := range p.peers {
		if !now.Before(entry.until) && entry.active == 0 {
			delete(p.peers, key)
		}
	}
	entry := p.peers[peer]
	if entry == nil {
		if len(p.peers) >= loginPeerLimit {
			return nil, loginWindow
		}
		entry = &loginAttempts{until: now.Add(loginWindow)}
		p.peers[peer] = entry
	}
	if !now.Before(entry.until) {
		entry.until, entry.failures = now.Add(loginWindow), 0
	}
	if entry.failures+entry.active >= loginFailureLimit {
		return nil, entry.until.Sub(now)
	}
	entry.active++
	return func(failed bool) {
		p.mu.Lock()
		defer p.mu.Unlock()
		entry.active--
		if failed {
			entry.failures++
		}
	}, 0
}
