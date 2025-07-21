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

	kopilotv1 "github.com/Fl0rencess720/Kopilot/api/v1"
	"github.com/Fl0rencess720/Kopilot/internal/controller/utils"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type LLMContent struct {
	Reason   string `json:"reason"`
	Solution string `json:"solution"`
}

type BotMessage struct {
	MsgType   string      `json:"msg_type"`
	Content   interface{} `json:"content"`
	Sign      string      `json:"sign,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
}

type PostContent struct {
	Post struct {
		ZhCn struct {
			Title   string       `json:"title"`
			Content [][]Elements `json:"content"`
		} `json:"zh_cn"`
	} `json:"post"`
}

type Elements struct {
	Tag      string `json:"tag"`
	Text     string `json:"text,omitempty"`
	Href     string `json:"href,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	Unescape bool   `json:"unescape,omitempty"`
}

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type FeishuSink struct {
	webhookURL string
	secret     string
}

func NewFeishuSink(clientset kubernetes.Interface, feishuSink kopilotv1.FeishuSink) (*FeishuSink, error) {
	webhookURL, err := utils.GetSecret(clientset, feishuSink.WebhookSecretRef.Key, feishuSink.WebhookSecretRef.Namespace, feishuSink.WebhookSecretRef.Name)
	if err != nil {
		return nil, err
	}
	secret, err := utils.GetSecret(clientset, feishuSink.SignatureSecretRef.Key, feishuSink.SignatureSecretRef.Namespace, feishuSink.WebhookSecretRef.Name)
	if err != nil {
		return nil, err
	}
	return &FeishuSink{
		webhookURL: webhookURL,
		secret:     secret,
	}, nil
}

func (s *FeishuSink) SendBotMessage(namespace, podName, content string) error {
	timestamp := time.Now().Unix()

	signature, err := genSign(s.secret, timestamp)
	if err != nil {
		zap.L().Error("gen signature failed", zap.Error(err))
		return err
	}

	message := BotMessage{
		Timestamp: strconv.FormatInt(timestamp, 10),
		Sign:      signature,
		MsgType:   "post",
		Content:   genPostContent(namespace, podName, content),
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		zap.L().Error("json marshal failed", zap.Error(err))
		return err
	}

	req, err := http.NewRequest("POST", s.webhookURL, bytes.NewBuffer(jsonData))
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

	var response Response

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

func genSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)

	h := hmac.New(sha256.New, []byte(stringToSign))

	signatureBytes := h.Sum(nil)

	signature := base64.StdEncoding.EncodeToString(signatureBytes)

	return signature, nil
}

func unmarshalLLMContent(content string) (LLMContent, error) {
	var llmContent LLMContent
	if err := json.Unmarshal([]byte(content), &llmContent); err != nil {
		zap.L().Error("unmarshal LLM content failed", zap.Error(err))
		return LLMContent{}, err
	}
	return llmContent, nil
}

func genPostContent(namespace, podName, content string) PostContent {
	llmContent, err := unmarshalLLMContent(content)
	if err != nil {
		zap.L().Error("unmarshal LLM content failed", zap.Error(err))
		return PostContent{}
	}

	postContent := PostContent{}
	postContent.Post.ZhCn.Title = "Kopilot Bot Alert"
	postContent.Post.ZhCn.Content = [][]Elements{
		{
			{
				Tag:  "text",
				Text: fmt.Sprintf("namespace: %s\npod: %s\n", namespace, podName),
			},
			{
				Tag:  "text",
				Text: fmt.Sprintf("reason: %s\nsolution: %s\n", llmContent.Reason, llmContent.Solution),
			},
		},
	}
	return postContent
}
