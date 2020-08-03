package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

type Response struct {
	Error   string
	Results []Video
}

type Video struct {
	Name        string
	PublishDate string `json:"publish_date"`
	HighURL     string `json:"high_url"`
	HdURL       string `json:"hd_url"`
}

const ua string = "GB Video Grabber Go/0.1.1"

var (
	re *regexp.Regexp = regexp.MustCompile(`[:?"]`)
)

func getInitialConfig() {
	videoDirDefault := "./videos/"
	concurrencyDefault := 3
	viper.SetDefault("videoDir", videoDirDefault)
	viper.SetDefault("apiKey", "")
	viper.SetDefault("maxConcurrency", concurrencyDefault)
	viper.SetDefault("offset", 0)
	viper.SetDefault("filter", nil)

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		viper.SafeWriteConfig()
	}

	viper.SetEnvPrefix("GBDL")
	viper.AutomaticEnv()

	pflag.String("videodir", videoDirDefault, "Directory in which to store downloaded videos")
	pflag.String("apikey", "", "Your GB API key")
	pflag.Int("maxconcurrency", concurrencyDefault, "Maximum number of concurrent downloads")
	pflag.Int("offset", 0, "Start from further back in history. E.g. --offset=100 will skip the most recent 100 videos and grab the next 100.")
	pflag.String("filter", "", "API filter to use. E.g. --filter=video_show:39")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	if viper.Get("apiKey") == "" {
		log.Fatal(`No API key was provided. You can solve this by doing one of the following:
		- Set it in your config.yaml file
		- Set a GBDL_APIKEY environment variable
		- Invoke the application with the --apikey=<your_key> flag`)
	}
}

func main() {
	getInitialConfig()

	url := fmt.Sprintf("https://www.giantbomb.com/api/videos/?api_key=%s&format=json&field_list=name,hd_url,high_url,publish_date", viper.GetString("apiKey"))

	if viper.GetInt("offset") > 0 {
		url += "&offset=" + viper.GetString("offset")
	}

	urlFilter := viper.GetString("filter")

	if urlFilter != "" {
		url += "&filter=" + urlFilter
	}

	gbClient := http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", ua)

	res, getErr := gbClient.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var responseObject Response
	json.Unmarshal(body, &responseObject)

	if responseObject.Error != "OK" {
		log.Fatal("API Error: ", responseObject.Error)
	}

	var wg sync.WaitGroup
	ch := make(chan Video)

	p := mpb.New(
		mpb.WithWaitGroup(&wg),
		mpb.WithWidth(60),
		mpb.WithRefreshRate(180*time.Millisecond),
	)

	for i := 0; i < viper.GetInt("maxConcurrency"); i++ {
		wg.Add(1)
		go videoWorker(ch, &wg, p)
	}

	for _, video := range responseObject.Results {
		ch <- video
	}

	close(ch)

	p.Wait()
}

func videoWorker(ch chan Video, wg *sync.WaitGroup, p *mpb.Progress) {
	for video := range ch {
		getVideo(video, p)
	}

	wg.Done()
}

func getVideo(video Video, p *mpb.Progress) {
	path := filepath.FromSlash(viper.GetString("videoDir"))

	err := os.MkdirAll(path, os.ModeDir|0775)
	if err != nil {
		log.Fatal(err)
	}

	date, err := time.Parse("2006-01-02 15:04:05", video.PublishDate)
	if err != nil {
		log.Fatal(err)
	}
	filename := fmt.Sprintf("%s %s.mp4", date.Format("200601021504"), strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(re.ReplaceAllString(video.Name, ""), "/", "-"), "|", "-"), "@", "at"))
	fullPath := path + filename
	hdURL := video.HdURL + "?api_key=" + viper.GetString("apiKey")

	dlClient := http.Client{}

	headResp, err := http.Head(hdURL)
	if err != nil {
		panic(err)
	}

	defer headResp.Body.Close()

	size, err := strconv.ParseInt(headResp.Header.Get("Content-Length"), 10, 0)
	if err != nil {
		panic(err)
	}

	out, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	fileStat, err := out.Stat()
	if err != nil {
		log.Fatal(err)
	}

	fileSize := fileStat.Size()

	if int64(size) <= fileSize {
		return
	}

	bar := p.AddBar(int64(size),
		mpb.PrependDecorators(
			decor.Name(video.Name+" "),
			decor.CountersKiloByte("% .2f / % .2f"),
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 90),
			decor.Name(" ] "),
			decor.EwmaSpeed(decor.UnitKB, "% .2f", 60),
		),
	)

	bar.SetCurrent(fileSize)

	req, err := http.NewRequest(http.MethodGet, hdURL, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", ua)
	if fileSize > 0 {
		setRangeHeader(req, fileSize)
	}

	resp, err := dlClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	proxyReader := bar.ProxyReader(resp.Body)
	defer proxyReader.Close()

	_, err = io.Copy(out, proxyReader)
	if err != nil {
		log.Fatal(err)
	}
}

func setRangeHeader(req *http.Request, size int64) {
	rangeString := fmt.Sprintf("bytes=%d-", size)
	req.Header.Set("Range", rangeString)
}
