package main

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

var (
	ipRegionOnce sync.Once
	ipSearcher   *xdb.Searcher
)

// initIP2Region 加载离线 xdb（vectorIndex 缓存模式）；库缺失只告警，归属地返回空
func initIP2Region(xdbPath string) {
	ipRegionOnce.Do(func() {
		handle, err := os.Open(xdbPath)
		if err != nil {
			log.Printf("ip2region 库缺失（%v），访客归属地将显示未知", err)
			return
		}
		defer handle.Close()
		header, err := xdb.LoadHeader(handle)
		if err != nil {
			log.Printf("ip2region 头读取失败: %v", err)
			return
		}
		ver, err := xdb.VersionFromHeader(header)
		if err != nil {
			log.Printf("ip2region 版本识别失败: %v", err)
			return
		}
		vIndex, err := xdb.LoadVectorIndexFromFile(xdbPath)
		if err != nil {
			log.Printf("ip2region 索引加载失败: %v", err)
			return
		}
		searcher, err := xdb.NewWithVectorIndex(ver, xdbPath, vIndex)
		if err != nil {
			log.Printf("ip2region 初始化失败: %v", err)
			return
		}
		ipSearcher = searcher
		log.Println("ip2region 归属地库已加载")
	})
}

// regionForIP 返回 "中国·广东省·深圳·电信" 形式；查不到返回空
func regionForIP(ip string) string {
	if ipSearcher == nil {
		return ""
	}
	raw, err := ipSearcher.Search(ip)
	if err != nil {
		return ""
	}
	parts := strings.Split(raw, "|")
	out := []string{}
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "0" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) >= 5 {
			break
		}
	}
	return strings.Join(out, "·")
}
