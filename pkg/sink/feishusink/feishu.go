package feishusink

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func GenSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)

	h := hmac.New(sha256.New, []byte(secret))
	_, err := h.Write([]byte(stringToSign))
	if err != nil {
		return "", err
	}
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return signature, nil
}

type BotMessage struct {
	Timestamp int64       `json:"timestamp"`
	Sign      string      `json:"sign"`
	MsgType   string      `json:"msg_type"`
	Content   interface{} `json:"content"`
}

type TextContent struct {
	Text string `json:"text"`
}

func SendBotMessage(webhookURL, secret, namespace, podName, content string) error {
	timestamp := time.Now().Unix()

	signature, err := GenSign(secret, timestamp)
	if err != nil {
		zap.L().Error("gen signature failed", zap.Error(err))
		return err
	}

	messageText := fmt.Sprintf("Namespace: %s\nPod: %s\n内容: %s", namespace, podName, content)

	message := BotMessage{
		Timestamp: timestamp,
		Sign:      signature,
		MsgType:   "text",
		Content: TextContent{
			Text: messageText,
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		zap.L().Error("json marshal failed", zap.Error(err))
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		zap.L().Error("http.NewRequest failed", zap.Error(err))
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		zap.L().Error("http.Do failed", zap.Error(err))
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zap.L().Error("close response body failed", zap.Error(err))
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zap.L().Error("response body read failed", zap.Error(err))
		return err
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("feishu api call failed, status code: %d, body: %s", resp.StatusCode, string(body))
		zap.L().Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		zap.L().Error("json unmarshal failed", zap.Error(err))
		return err
	}

	if response.Code != 0 {
		errMsg := fmt.Sprintf("feishu api call failed, code: %d, err: %s", response.Code, response.Msg)
		zap.L().Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	return nil
}
