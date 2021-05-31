package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// максимально допустимое число ошибок при парсинге
	errorsLimit = 100000

	// число результатов, которые хотим получить
	resultsLimit = 10000
)

var (
	// адрес в интернете (например, https://en.wikipedia.org/wiki/Lionel_Messi)
	url string

	// насколько глубоко нам надо смотреть (например, 10)
	depthLimit int
)

var Crawler *crawler

// Как вы помните, функция инициализации стартует первой
func init() {
	// задаём и парсим флаги
	flag.StringVar(&url, "url", "", "url address")
	flag.IntVar(&depthLimit, "depth", 3, "max depth for run")
	flag.Parse()

	// Проверяем обязательное условие
	if url == "" {
		log.Print("no url set by flag")
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func main() {
	startCrawler()
}

func startCrawler() {
	started := time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	go watchSignals(cancel)
	defer cancel()

	Crawler = newCrawler(depthLimit)

	// создаём канал для результатов
	results := make(chan crawlResult)

	// запускаем горутину для чтения из каналов
	done := watchCrawler(ctx, results, errorsLimit, resultsLimit)

	// запуск основной логики
	// внутри есть рекурсивные запуски анализа в других горутинах
	Crawler.run(ctx, url, results, 0)

	// ждём завершения работы чтения в своей горутине
	<-done

	log.Println(time.Since(started))
}

func liteCrawler() {
	started := time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	go watchSignals(cancel)
	defer cancel()

	Crawler = copyCrawler(Crawler, 2)

	results := make(chan crawlResult)

	done := watchCrawler(ctx, results, errorsLimit, resultsLimit)

	Crawler.run(ctx, url, results, 0)

	<-done

	log.Println(time.Since(started))
}

// ловим сигналы выключения
func watchSignals(cancel context.CancelFunc) {
	osSignalChan := make(chan os.Signal)

	go func() {
		for sig := range osSignalChan {
			switch sig {
			case syscall.SIGINT:
				log.Printf("got signal %q", sig.String())

				// если сигнал получен, отменяем контекст работы
				cancel()
			case syscall.SIGUSR1:
				liteCrawler()

			}
		}
	}()

	signal.Notify(osSignalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
}

func watchCrawler(ctx context.Context, results <-chan crawlResult, maxErrors, maxResults int) chan struct{} {
	readersDone := make(chan struct{})

	go func() {
		defer close(readersDone)
		for {
			select {
			case <-ctx.Done():
				return

			case result := <-results:
				if result.err != nil {
					maxErrors--
					if maxErrors <= 0 {
						log.Println("max errors exceeded")
						return
					}
					continue
				}

				log.Printf("crawling result: %v", result.msg)
				maxResults--
				if maxResults <= 0 {
					log.Println("got max results")
					return
				}
			}
		}
	}()

	return readersDone
}
