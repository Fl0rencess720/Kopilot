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
	"strconv"
	"time"

	"go.uber.org/zap"
)

func GenSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)

	h := hmac.New(sha256.New, []byte(stringToSign))

	signatureBytes := h.Sum(nil)

	signature := base64.StdEncoding.EncodeToString(signatureBytes)

	return signature, nil
}

type BotMessage struct {
	MsgType   string      `json:"msg_type"`
	Content   interface{} `json:"content"`
	Sign      string      `json:"sign,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
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
		Timestamp: strconv.FormatInt(timestamp, 10),
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
		err := fmt.Errorf("feishu api call failed, status code: %d, body: %s", resp.StatusCode, string(body))
		zap.L().Error(err.Error())
		return err
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
		err := fmt.Errorf("feishu api call failed, code: %d, err: %s", response.Code, response.Msg)
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
