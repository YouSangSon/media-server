package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v4"
)

func main() {
	router := gin.Default()
	router.MaxMultipartMemory = 8 << 20 // 8 MiB

	// 이미지 업로드 및 리사이징 엔드포인트
	router.POST("/upload-image", uploadImage)

	// 비디오 업로드 및 변환 엔드포인트
	router.POST("/upload-video", uploadVideo)

	// WebRTC 신호 처리 엔드포인트
	router.GET("/ws", signalPeerConnections)

	router.Run(":8080")
}

func uploadImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	filename := filepath.Base(file.Filename)
	if err := c.SaveUploadedFile(file, filename); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// 이미지 리사이징
	src, err := imaging.Open(filename)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("open image err: %s", err.Error()))
		return
	}

	dst := imaging.Resize(src, 800, 0, imaging.Lanczos)
	resizedFilename := "resized_" + filename
	err = imaging.Save(dst, resizedFilename)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("save image err: %s", err.Error()))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("Image uploaded and resized: %s", resizedFilename))
}

func uploadVideo(c *gin.Context) {
	file, err := c.FormFile("video")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	filename := filepath.Base(file.Filename)
	if err := c.SaveUploadedFile(file, filename); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// 비디오 변환
	outputFilename := "output.mp4"
	cmd := exec.Command("ffmpeg", "-i", filename, outputFilename)
	err = cmd.Run()
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("convert video err: %s", err.Error()))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("Video uploaded and converted: %s", outputFilename))
}

func signalPeerConnections(c *gin.Context) {
	// WebSocket 연결 설정
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create peer connection: %s", err.Error()))
		return
	}

	// ICE candidate 수신 처리
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		candidateJSON, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to marshal candidate: %s", err.Error()))
			return
		}
		c.Writer.Write(candidateJSON)
	})

	// offer 수신 처리
	offer := webrtc.SessionDescription{}
	err = c.BindJSON(&offer)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Failed to bind offer: %s", err.Error()))
		return
	}

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to set remote description: %s", err.Error()))
		return
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create answer: %s", err.Error()))
		return
	}

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to set local description: %s", err.Error()))
		return
	}

	answerJSON, err := json.Marshal(answer)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to marshal answer: %s", err.Error()))
		return
	}

	c.Writer.Write(answerJSON)
}
