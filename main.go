// main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	
	"gopkg.in/yaml.v3"
)



type Config struct {
	Targets []Target `yaml:"targets"`
	Pings   ping     `yaml:"ping"`
}

type Target struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	Name string `yaml:"name"`
}

type Statistics struct {
	minTime     float64
	maxTime     float64
	avgTime     float64
	lossPercent float64
	label       []string
}

type tcpingStatus struct {
	tcping_result  []float64
	sentCount      int
	respondedCount int
}

type ping struct {
	Interval int `yaml:"interval"`
	Timeout  int `yaml:"timeout"`
	Count    int `yaml:"count"`
}

// 指标结构体
type TcpingExporter struct {
	tcpingMetrics map[string]*prometheus.Desc
	mutex         sync.Mutex
}

func NewMetrics() *TcpingExporter {
	tcpingMetricsLabels := []string{
		"host",
		"port",
		"name",
	}

	tcpingMetrics := map[string]*prometheus.Desc{
		"tcping_loss_ratio": prometheus.NewDesc(
			"tcping_loss_ratio",
			"Tcping Packet loss from 0.0 to 100.0",
			tcpingMetricsLabels,
			nil),
		"tcping_rtt_best_ms": prometheus.NewDesc(
			"tcping_rtt_best_ms",
			"Tcping Best round trip time",
			tcpingMetricsLabels,
			nil),
		"tcping_rtt_worst_ms": prometheus.NewDesc(
			"tcping_rtt_worst_ms",
			"Tcping Worst round trip time",
			tcpingMetricsLabels,
			nil),
		"tcping_rtt_mean_ms": prometheus.NewDesc(
			"tcping_rtt_mean_ms",
			"Tcping Mean round trip time",
			tcpingMetricsLabels,
			nil),
	}

	return &TcpingExporter{
		tcpingMetrics: tcpingMetrics,
	}
}

/**
 * 接口：Describe
 * 功能：传递结构体中的指标描述符到channel
 */
func (c *TcpingExporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range c.tcpingMetrics {
		ch <- m
	}
}

/**
 * 接口：Collect
 * 功能：抓取最新的数据，传递给channel
 */
func (c *TcpingExporter) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock() // 加锁
	defer c.mutex.Unlock()

	yamlFile, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// 解析 YAML 数据
	var config Config
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}

	connectTimeoutFlag := config.Pings.Timeout
	intervalFlag := config.Pings.Interval
	countFlag := config.Pings.Count

	var wg sync.WaitGroup
	results := make(chan Statistics, len(config.Targets))

	for _, target := range config.Targets {
		wg.Add(1)
		go func(t Target) {
			defer wg.Done()
			ping_result := Tcping(t, countFlag, connectTimeoutFlag, intervalFlag)
			results <- ping_result
		}(target)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		tcpingMetricsLabels := result.label

		if result.lossPercent == 100.0 {
			ch <- prometheus.MustNewConstMetric(
				c.tcpingMetrics["tcping_loss_ratio"],
				prometheus.GaugeValue,
				result.lossPercent,
				tcpingMetricsLabels...,
			)
		} else {
			ch <- prometheus.MustNewConstMetric(
				c.tcpingMetrics["tcping_loss_ratio"],
				prometheus.GaugeValue,
				result.lossPercent,
				tcpingMetricsLabels...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.tcpingMetrics["tcping_rtt_best_ms"],
				prometheus.GaugeValue,
				result.minTime,
				tcpingMetricsLabels...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.tcpingMetrics["tcping_rtt_worst_ms"],
				prometheus.GaugeValue,
				result.maxTime,
				tcpingMetricsLabels...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.tcpingMetrics["tcping_rtt_mean_ms"],
				prometheus.GaugeValue,
				result.avgTime,
				tcpingMetricsLabels...,
			)
		}
	}
}

func printError(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

func computeResult(status tcpingStatus) (resultStatus Statistics) {
	if status.respondedCount != 0 {
		resultStatus = Statistics{minTime: float64(^uint(0) >> 1), maxTime: -float64(^uint(0) >> 1)}
		sum := 0.0
		// 计算平均值、最大值、最小值
		for _, value := range status.tcping_result {
			if value < resultStatus.minTime {
				resultStatus.minTime = value
			}
			if value > resultStatus.maxTime {
				resultStatus.maxTime = value
			}
			sum += value
		}
		resultStatus.avgTime = sum / float64(status.respondedCount)
	}
	resultStatus.lossPercent = 100.0 * float64(status.sentCount-status.respondedCount) / float64(status.sentCount)

	return
}

func Tcping(t Target, countFlag int, connectTimeoutFlag int, intervalFlag int) Statistics {
	// 获取内容
	address := t.Host
	port := t.Port
	var label []string
	label = append(label, t.Host)
	label = append(label, t.Port)
	label = append(label, t.Name)

	// 定义统计变量
	var status tcpingStatus
	// 遍历个数进行tcping
	for i := 0; i < countFlag; i++ {
		status.sentCount++
		start := time.Now()
		conn, err := net.DialTimeout("tcp",
			address+":"+port,
			time.Duration(connectTimeoutFlag)*time.Millisecond)
		elapsed := float64(time.Since(start).Microseconds()) / 1000.0 // 转换为毫秒的浮点数

		success := err == nil
		// log.Println(elapsed)

		if !success {
			log.Printf("Connection failed: %v", err)
			continue
		}

		status.respondedCount++
		status.tcping_result = append(status.tcping_result, elapsed)

		defer conn.Close()

		log.Printf("TCPing to %s:%s - time=%.3fms\n", address, port, elapsed)
		time.Sleep(time.Duration(intervalFlag) * time.Second)

	}

	log.Printf("TCPing status: sentCount=%d, respondedCount=%d, tcping_result=%v", 
    status.sentCount, status.respondedCount, status.tcping_result)


	tcping_result := computeResult(status)
	tcping_result.label = label

	return tcping_result
}

var (
	configFile  = flag.String("c", "", "Configuration file in YAML format")
	listenAddr  = flag.String("web.listen-port", "9379", "An port to listen on for web interface and telemetry.")
	metricsPath = flag.String("web.telemetry-path", "/metrics", "A path under which to expose metrics.")
)

func initLogger() {
	// 获取程序的执行路径
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	// 构建日志文件路径
	logDir := filepath.Dir(exePath)
	logFile := filepath.Join(logDir, "tcping-exporter.log")
	
	logger := &lumberjack.Logger{
		Filename:   logFile, // 日志文件路径
		MaxSize:    10,          // 单个日志文件的最大大小（单位：MB）
		MaxBackups: 3,           // 保留的旧日志文件个数
		MaxAge:     7,          // 保留的旧日志文件的最大天数
		Compress:   true,        // 是否压缩旧日志文件
	}

	// 将日志输出到 lumberjack 的 Writer
	log.SetOutput(logger)
}

func main() {
	flag.Parse()

	// 初始化日志记录器
	initLogger()

	if *configFile == "" {
		printError("Configuration file is required")
		os.Exit(1)
	}

	metrics := NewMetrics()
	registry := prometheus.NewRegistry()
	registry.MustRegister(metrics)

	http.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>A Prometheus Exporter
			</title></head>
			<body>
			<h1>A Prometheus Exporter</h1>
			<p><a href='/metrics'>Metrics</a></p>
			</body>
			</html>`))
	})

	log.Printf("Starting Server at http://localhost:%s%s", *listenAddr, "/metrics")
	log.Fatal(http.ListenAndServe(":"+*listenAddr, nil))

}
