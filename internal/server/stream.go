package server

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// StreamOptions 配置流式响应的行为
type StreamOptions struct {
	// ContentType 设置响应的内容类型，默认为"text/plain; charset=utf-8"
	ContentType string

	// CacheControl 设置缓存控制头，默认为"no-cache"
	CacheControl string

	// ErrorPrefix 设置错误信息的前缀，默认为"ERROR: "
	ErrorPrefix string
}

// TokenProducer 定义生成token的函数签名
type TokenProducer func(yield func(string) bool) error

// StreamPlainTokens 将 token 生产与网络写出解耦：
// - produce(yield) 通常在模型互斥锁内运行，但 yield 只负责入队，不做网络 I/O
// - writer goroutine 在锁外负责合并写出与 Flush，避免慢客户端放大锁持有时间
func StreamPlainTokens(c *gin.Context, produce TokenProducer, opts StreamOptions) (string, error) {
	// 设置默认值
	if opts.ContentType == "" {
		opts.ContentType = "text/plain; charset=utf-8"
	}
	if opts.CacheControl == "" {
		opts.CacheControl = "no-cache"
	}
	if opts.ErrorPrefix == "" {
		opts.ErrorPrefix = "ERROR: "
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return "", fmt.Errorf("streaming not supported")
	}

	c.Header("Content-Type", opts.ContentType)
	c.Header("Cache-Control", opts.CacheControl)
	c.Status(http.StatusOK)

	const (
		maxBufferedBytes = 2048
		flushInterval    = 40 * time.Millisecond
	)

	ctx := c.Request.Context()
	tokenCh := make(chan string, 256)
	doneCh := make(chan struct{})
	stopCh := make(chan struct{})

	var writerErr error
	go func() {
		defer close(doneCh)

		bw := bufio.NewWriterSize(c.Writer, 8*1024)
		var buf strings.Builder
		t := time.NewTicker(flushInterval)
		defer t.Stop()

		flush := func(force bool) bool {
			if writerErr != nil {
				return false
			}
			if buf.Len() == 0 && !force {
				return true
			}
			if buf.Len() == 0 {
				return true
			}
			if _, err := bw.WriteString(buf.String()); err != nil {
				writerErr = err
				close(stopCh)
				return false
			}
			if err := bw.Flush(); err != nil {
				writerErr = err
				close(stopCh)
				return false
			}
			flusher.Flush()
			buf.Reset()
			return true
		}

		for {
			select {
			case <-ctx.Done():
				writerErr = ctx.Err()
				close(stopCh)
				return
			case <-t.C:
				_ = flush(true)
			case token, ok := <-tokenCh:
				if !ok {
					_ = flush(true)
					return
				}
				if token == "" {
					continue
				}
				buf.WriteString(token)
				if buf.Len() >= maxBufferedBytes {
					_ = flush(true)
				}
			}
		}
	}()

	var out strings.Builder
	err := produce(func(token string) bool {
		select {
		case <-ctx.Done():
			return false
		case <-stopCh:
			return false
		default:
		}

		if token == "" {
			return true
		}
		out.WriteString(token)

		select {
		case tokenCh <- token:
			return true
		case <-ctx.Done():
			return false
		case <-stopCh:
			return false
		}
	})

	close(tokenCh)
	<-doneCh

	if writerErr != nil && err == nil {
		err = writerErr
	}

	if err != nil {
		if out.Len() == 0 {
			_, _ = c.Writer.WriteString(opts.ErrorPrefix + err.Error())
			flusher.Flush()
		}
		return out.String(), err
	}

	if out.Len() == 0 {
		select {
		case <-ctx.Done():
			return "", nil
		default:
		}
		errMsg := "Model produced no output (check logs for details)"
		_, _ = c.Writer.WriteString(opts.ErrorPrefix + errMsg)
		flusher.Flush()
		return "", fmt.Errorf("%s", errMsg)
	}

	return out.String(), nil
}

// WritePlainTokens 统一流式响应处理函数
// 该函数处理流式输出的所有通用逻辑，包括：
// - 检查HTTP Flushers支持
// - 设置响应头
// - 处理每个token的输出和flush
// - 处理取消和错误情况
// - 返回完整响应内容用于存储
func WritePlainTokens(c *gin.Context, produce TokenProducer, opts StreamOptions) (string, error) {
	// 设置默认值
	if opts.ContentType == "" {
		opts.ContentType = "text/plain; charset=utf-8"
	}
	if opts.CacheControl == "" {
		opts.CacheControl = "no-cache"
	}
	if opts.ErrorPrefix == "" {
		opts.ErrorPrefix = "ERROR: "
	}

	// 检查是否支持流式输出
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return "", fmt.Errorf("streaming not supported")
	}

	// 设置响应头
	c.Header("Content-Type", opts.ContentType)
	c.Header("Cache-Control", opts.CacheControl)
	c.Status(http.StatusOK)

	// 创建字符串构建器来收集完整响应
	var out strings.Builder

	// 写出缓冲：减少每 token Flush 的 syscall/网络包
	// - 超过 maxBufferedBytes 或超过 flushInterval 时才 flush 一次
	const (
		maxBufferedBytes = 2048
		flushInterval    = 40 * time.Millisecond
	)
	var (
		buf       strings.Builder
		lastFlush = time.Now()
		writeErr  error
	)

	flushBuffered := func(force bool) bool {
		if writeErr != nil {
			return false
		}
		if buf.Len() == 0 && !force {
			return true
		}
		if !force && buf.Len() < maxBufferedBytes && time.Since(lastFlush) < flushInterval {
			return true
		}
		if buf.Len() == 0 {
			return true
		}
		if _, err := c.Writer.WriteString(buf.String()); err != nil {
			writeErr = err
			return false
		}
		flusher.Flush()
		buf.Reset()
		lastFlush = time.Now()
		return true
	}

	// 执行token生产函数
	err := produce(func(token string) bool {
		// 检查请求是否已取消
		select {
		case <-c.Request.Context().Done():
			return false
		default:
		}

		// 过滤空token
		if token == "" {
			return true
		}

		// 累积token到完整响应中
		out.WriteString(token)

		// 写入缓冲（定期 flush）
		buf.WriteString(token)
		if !flushBuffered(false) {
			// 写失败（通常是客户端断开），中止生成
			return false
		}

		return true
	})

	// produce 结束后 flush 余下内容
	_ = flushBuffered(true)

	if writeErr != nil && err == nil {
		// 客户端断连/写失败：将其作为错误返回，便于上层判断
		err = writeErr
	}

	// 处理错误情况
	if err != nil {
		// 如果没有输出任何内容，则显示错误信息
		if out.Len() == 0 {
			_, _ = c.Writer.WriteString(opts.ErrorPrefix + err.Error())
			flusher.Flush()
		}
		return out.String(), err
	}

	// 如果没有输出任何内容且没有错误，这也是一种异常情况
	if out.Len() == 0 {
		select {
		case <-c.Request.Context().Done():
			return "", nil
		default:
		}
		errMsg := "Model produced no output (check logs for details)"
		_, _ = c.Writer.WriteString(opts.ErrorPrefix + errMsg)
		flusher.Flush()
		return "", fmt.Errorf("%s", errMsg)
	}

	return out.String(), nil
}
