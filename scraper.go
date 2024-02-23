package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

func main() {
	//запись текущего времени
	start := time.Now()
	//список акций
	stocks := []string{
		"AAPL",
		"MSFT",
		"AMZN",
		"PLTR",
	}

	//вызов функции парсинга для списка акций
	err := parseStocks(stocks)
	if err != nil {
		log.Fatalf("Error parsing stocks: %v", err)
	}
	//вывод времени хода выполнения
	fmt.Printf("Completed the code process, took: %f seconds", time.Since(start).Seconds())
}

func parseStocks(stocks []string) error {
	//создание канала для обмена сообщениями между горутинами
	ch := make(chan string)
	//создание группу ожидания для синхронизации горутин
	var wg sync.WaitGroup
	var mu sync.Mutex

	//запуск паринга каждой акции в отдельной горутине
	for _, stock := range stocks {
		wg.Add(1)
		go parseStock(stock, ch, &wg, &mu)
	}

	//закрытие канала, когда горутины закончат свою работу
	go func() {
		wg.Wait()
		close(ch)
	}()

	//чтение сообщений из канала и вывод их на экран
	for msg := range ch {
		fmt.Println(msg)
	}
	return nil
}

func parseStock(stock string, ch chan string, wg *sync.WaitGroup, mu *sync.Mutex) {
	//уменьшение счётчика группы ожидания при завершении функции
	defer (*wg).Done()

	//создание нового коллектора
	c := colly.NewCollector(
		//пользовательский агент
		colly.UserAgent("1 Mozilla/5.0 (iPad; CPU OS 12_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148"),
		//разрешенные домены
		colly.AllowedDomains("finance.yahoo.com"),
		//максимальный размер тела ответа
		colly.MaxBodySize(0),
		//разрешение повторного запроса URL
		colly.AllowURLRevisit(),
		//асинхронный режим
		colly.Async(true),
	)

	//Ограничение параллелизма и ввод случайной задержки
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
	})

	//обработка ошибок при посещении страницы
	c.OnError(func(r *colly.Response, err error) {
		//защита доступа к ошибке с помощью мьютекса
		(*mu).Lock()
		defer (*mu).Unlock()
		log.Printf("Error parsing stock %s: %v", stock, err)
	})
	//логирование URL перед отправкой запроса
	c.OnRequest(func(h *colly.Request) {
		log.Println("Visiting", h.URL.String())
	})
	//извлечение данных из HTML элемента
	c.OnHTML("tbody", func(e *colly.HTMLElement) {
		e.ForEach("tr", func(_ int, el *colly.HTMLElement) {
			dataSlice := []string{}
			el.ForEach("td", func(_ int, el *colly.HTMLElement) {
				dataSlice = append(dataSlice, el.Text)
			})
			// Проверка, содержит ли первый элемент "Previous Close" и отправляем данные в канал
			if dataSlice[0] == "Previous Close" {
				ch <- stock + " Price for previous close is: " + dataSlice[1]
			}
		})
	})
	//посещение URL для каждой акции
	err := c.Visit("https://finance.yahoo.com/quote/" + stock)
	if err != nil {
		(*mu).Lock()
		defer (*mu).Unlock()
		log.Printf("Error visiting stock %s: %v", stock, err)
	}
	//ожидание завершения всех запросов
	c.Wait()

}
