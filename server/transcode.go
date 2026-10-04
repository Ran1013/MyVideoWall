package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// 上传后自动压缩：把录制级大码率视频压成适合小带宽流播放的规格
//
//	参数来自 TranscodeStore（管理端可调，默认 720p / 30fps / CRF28 / 码率上限 1.2Mbps / 单文件上限 4GB）
//	单线程互斥：同一时间只压一个，避免 2C 小机器被撑爆
const targetEdgePx = 1280 // 默认长边上限（720p）

var transcodeMu sync.Mutex

func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// transcodeForWeb 原地压缩 src（产物一律 H.264 MP4）。
// 返回 replaced=true 表示原文件已被转码产物替换（此时内容与扩展名可能不符，调用方应改名为 .mp4）；
// 失败返回错误且保留原文件。
func transcodeForWeb(src string, tc *TranscodeStore) (bool, error) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return false, fmt.Errorf("ffmpeg 不可用")
	}
	info, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	p := tc.Get()
	if p.MaxMB > 0 && info.Size() > int64(p.MaxMB)*1024*1024 {
		return false, fmt.Errorf("文件 %dMB 超过转码上限 %dMB，跳过", info.Size()/1048576, p.MaxMB)
	}
	// 磁盘余量保护：源文件与转码临时文件并存，余量不足文件大小 2 倍时跳过（防撑爆磁盘殃及 MySQL）
	var st syscall.Statfs_t
	if statErr := syscall.Statfs(filepath.Dir(src), &st); statErr == nil {
		free := int64(st.Bavail) * int64(st.Bsize)
		if free < info.Size()*2 {
			return false, fmt.Errorf("磁盘剩余仅 %.1fGB，不足 %.1fGB 文件的 2 倍，跳过压缩", float64(free)/1073741824, float64(info.Size())/1073741824)
		}
	}

	transcodeMu.Lock()
	defer transcodeMu.Unlock()

	ext := filepath.Ext(src)
	dst := strings.TrimSuffix(src, ext) + ".transcoding.mp4"

	maxrate := fmt.Sprintf("%dk", p.MaxrateK)
	bufsize := fmt.Sprintf("%dk", p.MaxrateK*2)
	fps := strconv.Itoa(p.Fps)
	crf := strconv.Itoa(p.Crf)

	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", src,
		// force_divisible_by=2：手机竖屏等奇数高度视频，x264 要求宽高必须是偶数
		"-vf", fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2", p.EdgePx, p.EdgePx),
		"-r", fps,
		"-c:v", "libx264", "-crf", crf, "-preset", "veryfast",
		"-maxrate", maxrate, "-bufsize", bufsize,
		"-c:a", "aac", "-b:a", "96k",
		"-movflags", "+faststart",
		dst,
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		os.Remove(dst)
		return false, fmt.Errorf("ffmpeg 失败: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	fi, err := os.Stat(dst)
	if err != nil || fi.Size() < 1024 {
		os.Remove(dst)
		return false, fmt.Errorf("转码产物无效")
	}
	// 产物明显更小才替换（避免把已优化的小文件变大）
	if fi.Size() >= info.Size() {
		os.Remove(dst)
		log.Printf("转码产物 %dKB 不小于原文件 %dKB，保留原文件", fi.Size()/1024, info.Size()/1024)
		return false, nil
	}

	// 直接 rename 覆盖（原子操作，别先删后改——中途失败会两头空）
	if err := os.Rename(dst, src); err != nil {
		os.Remove(dst)
		return false, err
	}
	log.Printf("压缩完成：%dMB → %dMB", info.Size()/1048576, fi.Size()/1048576)
	return true, nil
}

// generatePoster 从视频抽一帧当封面（<video>.jpg），首页卡片只加载这张小图，避免抢占带宽
func generatePoster(videoPath string) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return
	}
	dst := videoPath + ".jpg"
	if _, err := os.Stat(dst); err == nil {
		return // 已有封面（用户上传过的同名 jpg）不覆盖
	}
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-ss", "1", "-i", videoPath,
		"-frames:v", "1", "-vf", "scale=640:-2",
		"-q:v", "4", dst,
	}
	if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
		os.Remove(dst)
		log.Printf("封面生成失败：%v (%s)", err, strings.TrimSpace(string(out)))
		return
	}
	log.Printf("封面已生成：%s", filepath.Base(dst))
}
