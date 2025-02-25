# tcping-exporter

TCP Ping 监控，通过IP 和端口，自定义次数和间隔时间，采集IP和端口的延时和丢包数据。
### 使用方法
```
./tcping-exporter --help
Usage of ./tcping-exporter:
  -c string
        Configuration file in YAML format
  -web.listen-port string
        An port to listen on for web interface and telemetry. (default "9379")
  -web.telemetry-path string
        A path under which to expose metrics. (default "/metrics")

```
### config.yml 配置样例
```
targets:
  - host: 127.0.0.1
    port: 443
    name: 测试label

ping:
  interval: 1
  timeout: 1000
  count: 4
```
### Exported metrics
```
# HELP tcping_loss_ratio Tcping Packet loss from 0.0 to 100.0
# TYPE tcping_loss_ratio gauge
tcping_loss_ratio{host="127.0.0.1",name="测试label",port="443"} 0
# HELP tcping_rtt_best_ms Tcping Best round trip time
# TYPE tcping_rtt_best_ms gauge
tcping_rtt_best_ms{host="127.0.0.1",name="测试label",port="443"} 51.788
# HELP tcping_rtt_mean_ms Tcping Mean round trip time
# TYPE tcping_rtt_mean_ms gauge
tcping_rtt_mean_ms{host="127.0.0.1",name="测试label",port="443"} 53.747749999999996
# HELP tcping_rtt_worst_ms Tcping Worst round trip time
# TYPE tcping_rtt_worst_ms gauge
tcping_rtt_worst_ms{host="127.0.0.1",name="测试label",port="443"} 55.576
```

### prometheus 配置
```
- job_name: "tcping-exporter"
    scrape_interval: 15s
    scrape_timeout: 15s
    static_configs:
      - targets: ["127.0.0.1:9379"]
        labels:
          hostname: tcping-host

```
