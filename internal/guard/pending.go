package guard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrConfirmExpired  = errors.New("guard: confirmation expired or unknown")
	ErrConfirmNotOwner = errors.New("guard: confirmation belongs to another user")
)

type Request struct {
	Call      Call
	Reason    string
	Sensitive bool
}

func (r Request) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "¿Ejecuto %s?", r.Call.Skill)
	if r.Call.Server != "" {
		fmt.Fprintf(&b, " (servidor %s)", r.Call.Server)
	}
	switch args := redactArgs(r.Call.ArgsJSON); {
	case r.Sensitive:
		b.WriteString("\nArgumentos: [ocultos: contienen información sensible]")
	case args != nil:
		raw, _ := json.MarshalIndent(args, "", "  ")
		fmt.Fprintf(&b, "\nArgumentos:\n%s", raw)
	}
	if r.Reason != "" {
		fmt.Fprintf(&b, "\nMotivo: %s", r.Reason)
	}
	return b.String()
}

type Pending struct {
	mu sync.Mutex
	m  map[string]pendingReq
}

type pendingReq struct {
	owner string
	ch    chan bool
}

func NewPending() *Pending {
	return &Pending{m: map[string]pendingReq{}}
}

func (p *Pending) Add(owner string) (string, <-chan bool) {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	id := hex.EncodeToString(buf)
	ch := make(chan bool, 1)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[id] = pendingReq{owner: owner, ch: ch}
	return id, ch
}

func Await(ctx context.Context, ch <-chan bool) (bool, error) {
	select {
	case answer := <-ch:
		return answer, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (p *Pending) Resolve(id, who string, answer bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	req, ok := p.m[id]
	if !ok {
		return ErrConfirmExpired
	}
	if req.owner != who {
		return ErrConfirmNotOwner
	}
	delete(p.m, id)
	req.ch <- answer
	return nil
}

func (p *Pending) Drop(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, id)
}
