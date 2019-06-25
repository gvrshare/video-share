package seed

import (
	"context"
	shell "github.com/godcong/go-ipfs-restapi"
	"sync"

	"github.com/yinhevr/seed/model"
)

// PinFlag ...
type PinFlag string

// PinFlagNone ...
const (
	PinFlagNone PinFlag = "none"
	//PinFlagPoster PinFlag = "poster"
	PinFlagSource PinFlag = "source"
	PinFlagSlice  PinFlag = "slice"
	PinFlagAll    PinFlag = "all"
)

type pin struct {
	wg         *sync.WaitGroup
	unfinished map[string]*model.Unfinished
	shell      *shell.Shell
	state      PinStatus
	flag       PinFlag
	status     PinStatus
}

// BeforeRun ...
func (p *pin) BeforeRun(seed *Seed) {
	p.unfinished = seed.Unfinished
	if p.unfinished == nil {
		p.unfinished = make(map[string]*model.Unfinished)
	}
	if p.shell == nil {
		p.shell = seed.Shell
	}

}

// AfterRun ...
func (p *pin) AfterRun(seed *Seed) {
	return
}

// PinStatus ...
type PinStatus string

// PinStatusAll ...
const PinStatusAll PinStatus = "all"

// PinStatusBefore ...
const PinStatusBefore PinStatus = "before"

// PinStatusAssign ...
const PinStatusAssign PinStatus = "assign"

// Pin ...
func Pin(status PinStatus) Options {
	pin := &pin{
		status: status,
		wg:     &sync.WaitGroup{},
	}

	return pinOption(pin)
}

// Run ...
func (p *pin) Run(ctx context.Context) {
	log.Info("pin running")
	switch p.status {
	case PinStatusAll:
		unfins, e := model.AllUnfinished(nil, 0)
		if e != nil {
			return
		}
		for _, unf := range *unfins {
			go p.pinHash(unf.Hash)
		}
	}

	for hash := range p.unfinished {
		select {
		case <-ctx.Done():
			return
		default:
			p.wg.Add(1)
			go p.pinHash(hash)
			p.wg.Wait()
			p.unfinished[hash].Sync = true
		}
	}
}

func (p *pin) pinHash(hash string) {
	log.Info("pin:", hash)
	defer func() {
		if p.wg != nil {
			p.wg.Done()
		}
	}()
	e := p.shell.Pin(hash)
	if e != nil {
		log.Error("pin error:", hash, e)
		return
	}

	log.Info("pinned:", hash)
}
