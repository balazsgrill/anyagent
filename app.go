package main

import (
	"flag"
	"log"
	"time"

	"github.com/anyproto/anytype-heart/pb"
	"github.com/balazsgrill/extractld"
	extractldapp "github.com/balazsgrill/extractld/app"
)

type Main struct {
	extractldapp.ExtractorApp
	config AgentConfig
	agent  *agent

	ticker         *time.Ticker
	tickerstop     chan bool
	mailprocessors map[extractld.MailProvider]*MailProcessor
}

func (m *Main) InitFlags() {
	m.ExtractorApp.InitFlags()
	flag.StringVar(&m.config.Storagepath, "p", "", "storage path")
	flag.StringVar(&m.config.Mnemonic, "m", "", "Secret mnemonic")
	flag.StringVar(&m.config.Accountid, "a", "", "Account ID")
}

func (m *Main) ParseFlags() {
	m.ExtractorApp.ParseFlags()
	if m.config.Storagepath == "" {
		panic("No storage path has been configured")
	}
	if m.config.Mnemonic == "" {
		panic("Mnemonic has not been configured")
	}
	if m.config.Accountid == "" {
		panic("Account ID has not been configured")
	}
}

func (m *Main) Init() {
	m.ExtractorApp.Init()
	m.mailprocessors = make(map[extractld.MailProvider]*MailProcessor)
	m.agent = New(m.config)
	m.agent.Init()
}

func (m *Main) Stop() {
	m.ticker.Stop()
	m.tickerstop <- true
}

func (m *Main) tickerProcess() {
	for {
		select {
		case <-m.tickerstop:
			return
		case now := <-m.ticker.C:
			m.tick(now)
		}
	}
}

func (m *Main) mailProcessor(mp extractld.MailProvider, now time.Time) *MailProcessor {
	p, ok := m.mailprocessors[mp]
	if !ok {
		p = &MailProcessor{
			MailProvider: mp,
			lastTime:     now.Add(-14 * 24 * time.Hour), // Two weeks
		}
	}
	return p
}

func (m *Main) tick(now time.Time) {
	var toImport []*pb.RpcObjectImportRequestSnapshot
	for _, mp := range m.ExtractorApp.MailProviders() {
		toImport = append(toImport, m.mailProcessor(mp, now).Do(now)...)
	}
	if len(toImport) > 0 {
		err := m.agent.ImportSnapshots(toImport)
		if err != nil {
			log.Println(err)
		}
	}
}

func (m *Main) Start() {
	m.agent.Start()
	m.ticker = time.NewTicker(time.Minute)
	m.tickerstop = make(chan bool)
	go m.tickerProcess()
	m.ExtractorApp.Start()
}
