package main

import (
	"bufio"
	"fmt"
	"github.com/panjf2000/ants"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2" // imports as package "cli"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	/*"github.com/Konstantin8105/DDoS"*/
)

func main()  {
	cli.AppHelpTemplate = fmt.Sprintf(`NAME:
   {{.Name}}{{if .Usage}} - {{.Usage}}{{end}}

GLOBAL OPTIONS:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}

AUTHOR: Libs

	`)
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "threads",
				Value: "1",
				Usage: "线程数",
			},
			&cli.StringFlag{
				Name:  "proxy_file_path",
				Value: "",
				Required: true,
				Usage: "代理文本文件路径",
			},
			&cli.StringFlag{
				Name:  "ua_file_path",
				Value: "",
				Required: true,
				Usage: "随机ua文本文件路径",
			},
			&cli.StringFlag{
				Name:  "test_url",
				Value: "",
				Required: true,
				Usage: "代理后访问的网站，$RAND$替换随机字符串",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Value: false,
				Required: false,
				Usage: "是否开启debug模式",
			},
		},
		Name : "GetUrl",
		Usage: "循环使用随机ua和代理请求地址",
		Action: func(c *cli.Context) error {
			threads := c.Int("threads")
			proxyFilePath := c.String("proxy_file_path")
			uaFilePath := c.String("ua_file_path")
			testUrl := c.String("test_url")
			debug := c.Bool("debug")
			if debug{
				log.SetLevel(log.DebugLevel)
			}
			run(threads, proxyFilePath, uaFilePath, testUrl)
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
func run(threads int, proxyFilePath, uaFilePath, testUrl  string) {
	uas := GetUas(uaFilePath)
	testUrl = HandleUrl(testUrl)
	rand.Seed(time.Now().Unix())
	var wg sync.WaitGroup
	defer ants.Release()
	log.Infof("[%s]循环请求开始...", proxyFilePath)
	strCh := make(chan string)
	for i := 0; i < threads; i++ {
		wg.Add(1)
		ants.Submit(func() { //提交函数，将逻辑函数提交至work中执行，这里写入自己的逻辑
			defer wg.Done()
			for {
				line, ok := <- strCh
				if !ok {
					break
				}
				for i := 0; i < 50; i++ {
					ProxyGet("http://" + line, testUrl, uas)
				}
			}
		})
	}
	GetLines(proxyFilePath, strCh)
	close(strCh)
	wg.Wait()
	log.Infof("[%s]检测结束...", proxyFilePath)
}
func HandleUrl(testUrl string) string{
	if strings.Contains(testUrl,"$RAND$"){
		testUrl = strings.ReplaceAll(testUrl, "$RAND$", GetRandomString(8))
		log.Info("替换后的测试地址：" + testUrl)
	}
	return testUrl
}
func  GetRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}
func GetLines(filename string, strCh chan string){
	i := 0
	for {
		i++
		log.Infof("第%d次循环...", i)
		file, _ := os.Open(filename)
		defer file.Close()
		reader := bufio.NewReader(file)
		for {
			lineBytes, _, err := reader.ReadLine()
			lineStr := string(lineBytes)
			lineStr = strings.TrimSpace(lineStr)
			hostAndPort := strings.Split(lineStr,":")
			if len(hostAndPort) < 2{
				//log.Infof("[%s]不符合ip:port的格式", lineStr)
			}else{
				strCh <- lineStr
			}
			if err == io.EOF {
				break
			}
		}
	}
}
func GetUas(uaFilePath string) []string{
	uas := []string{}
	file, _ := os.Open(uaFilePath)
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		lineBytes, _, err := reader.ReadLine()
		ua := string(lineBytes)
		uas = append(uas, ua)
		if err == io.EOF {
			break
		}
	}
	return uas
}
func WriteCheckResults(outFileName string, alivable []string)  {
	for _, line := range alivable{
		fd,_:=os.OpenFile(outFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND,0644)
		buf:=[]byte(line + "\n")
		fd.Write(buf)
		fd.Close()
	}
}
func WriteFile(outFileName, line string) {
	fd,_:=os.OpenFile(outFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND,0644)
	buf:=[]byte("\n" + line)
	fd.Write(buf)
	fd.Close()
}
func ProxyGet(proxy_addr, testUrl string, uas []string) (speed int, result bool){
	// 解析代理地址
	proxy, err := url.Parse(proxy_addr)
	if err!= nil {
		return -1, false
	}
	//设置网络传输
	netTransport := &http.Transport{
		Proxy:                 http.ProxyURL(proxy),
		MaxIdleConnsPerHost:   5,
		ResponseHeaderTimeout: time.Second * time.Duration(5),
		DisableKeepAlives: false,
	}
	// 创建连接客户端
	httpClient := &http.Client{
		Timeout:   time.Second * 5,
		Transport: netTransport,
	}
	rand.Seed(time.Now().Unix())
	//begin := time.Now() //判断代理访问时间
	// 使用代理IP访问测试地址
	req, _ := http.NewRequest("GET", testUrl, nil)
	ua := uas[rand.Intn(len(uas))]
	req.Header.Set("User-Agent", ua)
	//res, err := httpClient.Get(testUrl)
	res, err := httpClient.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return -1,false
	}
	//speed = int(time.Now().Sub(begin).Nanoseconds() / 1000 / 1000) //ms
	if res.StatusCode == 200{
		return speed, true
	}
	return speed, false
}
func ShortDur(d time.Duration) string {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", d.Minutes()), 64)
	return fmt.Sprint(v) + "m"
}