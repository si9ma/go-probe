package probe

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

type stat struct {
	urlSucceedCount int
	urlFailedCount  int
	endTime         time.Time
	startTime       time.Time
}

// Probe probe
type Probe struct {
	urlChan chan string
	resChan chan http.Response
	guard   chan struct{}
	done    chan struct{}
	stat    stat
	temp    int
}

func (probe *Probe) runGenURLTask() {
	for i := 0; i < 500; i++ {
		probe.urlChan <- "https://www.baidu.com"
	}
	close(probe.urlChan)
}

func (probe *Probe) runDownloadTask() {
	wg := sync.WaitGroup{}
	for url := range probe.urlChan {
		probe.guard <- struct{}{}
		wg.Add(1)
		go probe.downloadTask(url, &wg)
	}
	wg.Wait()
	close(probe.resChan)
}

func (probe *Probe) downloadTask(url string, wg *sync.WaitGroup) {
	defer func() {
		<-probe.guard
		wg.Done()
	}()
	res, err := getRes(url)
	if err != nil {
		log.Println(err)
		probe.stat.urlFailedCount++
		return
	}
	probe.resChan <- *res
	probe.stat.urlSucceedCount++
}

func getRes(url string) (*http.Response, error) {
	client := http.Client{}
	header := http.Header{}
	header.Set("User-Agent", "probe 0.0.1")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{
		Name:       "",
		Value:      "",
		Path:       "",
		Domain:     "",
		Expires:    time.Time{},
		RawExpires: "",
		MaxAge:     0,
		Secure:     false,
		HttpOnly:   false,
		SameSite:   0,
		Raw:        "",
		Unparsed:   []string{},
	})
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (probe *Probe) runSaveDataTask() {
	for res := range probe.resChan {
		probe.Save(res)
		_ = res.Body.Close()
	}
	close(probe.done)
}

// Save saves data
func (probe *Probe) Save(res http.Response) {
	// fmt.Println(res.Status)
	if probe.stat.urlSucceedCount == 1 {
		_, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return
		}
	}
}

func (probe *Probe) printFinal() {
	deltaTime := time.Now().Sub(probe.stat.startTime)
	speed := float64(probe.stat.urlSucceedCount) / deltaTime.Minutes()
	fmt.Printf("Fetched URL: %d, Failed: %d [ %.2f%% ]\n", probe.stat.urlSucceedCount, probe.stat.urlFailedCount, probe.rate())
	fmt.Printf("Speed: %.2f req/min, URL: %d\n", speed, probe.stat.urlSucceedCount+probe.stat.urlFailedCount)
}

func (probe *Probe) runLoggingTask() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			fmt.Printf("Fetched URL: %d, Failed: %d [ %.2f%% ]\n", probe.stat.urlSucceedCount, probe.stat.urlFailedCount, probe.rate())
		case <-probe.done:
			return
		}
	}
}

func (probe *Probe) rate() float64 {
	rate := 0.0
	if probe.stat.urlSucceedCount+probe.stat.urlFailedCount != 0 {
		rate = float64(probe.stat.urlSucceedCount) / float64(probe.stat.urlSucceedCount+probe.stat.urlFailedCount) * 100
	}
	return rate
}

// Run run the probe
func (probe *Probe) Run() {
	probe.stat.startTime = time.Now()
	defer probe.printFinal()
	go probe.runGenURLTask()
	go probe.runDownloadTask()
	go probe.runSaveDataTask()
	probe.runLoggingTask()
}

// NewProbe generates new Probe
func NewProbe() *Probe {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	return &Probe{
		urlChan: make(chan string),
		resChan: make(chan http.Response),
		guard:   make(chan struct{}, 128),
		done:    make(chan struct{}),
	}
}
