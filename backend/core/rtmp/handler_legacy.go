package rtmp

//
// import (
// 	"bytes"
// 	"fmt"
// 	"io"
// 	"log"
// 	"os"
// 	"path/filepath"
// 	"sen1or/lets-live/internal/config"
//
// 	"github.com/pkg/errors"
// 	"github.com/yutopp/go-flv"
// 	flvtag "github.com/yutopp/go-flv/tag"
// 	"github.com/yutopp/go-rtmp"
// 	rtmpmsg "github.com/yutopp/go-rtmp/message"
// )
//
// var _ rtmp.Handler = (*Handler)(nil)
//
// type Handler struct {
// 	rtmp.DefaultHandler
// 	flvFile *os.File
// 	flvEnc  *flv.Encoder
// 	config  config.Config
// }
//
// func (h *Handler) OnServe(conn *rtmp.Conn) {
// }
//
// func (h *Handler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
// 	log.Printf("OnConnect: %#v", cmd)
// 	return nil
// }
//
// func (h *Handler) OnCreateStream(timestamp uint32, cmd *rtmpmsg.NetConnectionCreateStream) error {
// 	log.Printf("OnCreateStream: %#v", cmd)
// 	return nil
// }
//
// func (h *Handler) OnPublish(_ *rtmp.StreamContext, timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
// 	log.Printf("OnPublish: %#v", cmd)
//
// 	// (example) Reject a connection when PublishingName is empty
// 	if cmd.PublishingName == "" {
// 		return errors.New("PublishingName is empty")
// 	}
//
// 	privateWorkingDir := filepath.Join(h.config.PrivateHLSPath, cmd.PublishingName)
// 	publicWorkingDir := filepath.Join(h.config.PublicHLSPath, cmd.PublishingName)
// 	os.RemoveAll(privateWorkingDir)
// 	os.MkdirAll(privateWorkingDir, 0777)
// 	os.RemoveAll(publicWorkingDir)
// 	os.MkdirAll(publicWorkingDir, 0777)
//
// 	// Record streams as FLV!
// 	pipePath := filepath.Join(
// 		os.TempDir(),
// 		filepath.Clean(filepath.Join("/", fmt.Sprintf("%s.flv", cmd.PublishingName))),
// 	)
//
// 	f, err := os.OpenFile(pipePath, os.O_CREATE|os.O_WRONLY, 0666)
// 	if err != nil {
// 		return errors.Wrap(err, "Failed to create flv file")
// 	}
// 	h.flvFile = f
//
// 	enc, err := flv.NewEncoder(f, flv.FlagsAudio|flv.FlagsVideo)
// 	if err != nil {
// 		_ = f.Close()
// 		return errors.Wrap(err, "Failed to create flv encoder")
// 	}
// 	h.flvEnc = enc
//
// 	go startFfmpeg(pipePath, cmd.PublishingName)
//
// 	return nil
// }
//
// func (h *Handler) OnSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {
// 	r := bytes.NewReader(data.Payload)
//
// 	var script flvtag.ScriptData
// 	if err := flvtag.DecodeScriptData(r, &script); err != nil {
// 		log.Printf("Failed to decode script data: Err = %+v", err)
// 		return nil // ignore
// 	}
//
// 	log.Printf("SetDataFrame: Script = %#v", script)
//
// 	if err := h.flvEnc.Encode(&flvtag.FlvTag{
// 		TagType:   flvtag.TagTypeScriptData,
// 		Timestamp: timestamp,
// 		Data:      &script,
// 	}); err != nil {
// 		log.Printf("Failed to write script data: Err = %+v", err)
// 	}
//
// 	return nil
// }
//
// func (h *Handler) OnAudio(timestamp uint32, payload io.Reader) error {
// 	var audio flvtag.AudioData
// 	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
// 		return err
// 	}
//
// 	flvBody := new(bytes.Buffer)
// 	if _, err := io.Copy(flvBody, audio.Data); err != nil {
// 		return err
// 	}
// 	audio.Data = flvBody
//
// 	if err := h.flvEnc.Encode(&flvtag.FlvTag{
// 		TagType:   flvtag.TagTypeAudio,
// 		Timestamp: timestamp,
// 		Data:      &audio,
// 	}); err != nil {
// 		log.Printf("Failed to write audio: Err = %+v", err)
// 	}
//
// 	return nil
// }
//
// func (h *Handler) OnVideo(timestamp uint32, payload io.Reader) error {
// 	var video flvtag.VideoData
// 	if err := flvtag.DecodeVideoData(payload, &video); err != nil {
// 		return err
// 	}
//
// 	flvBody := new(bytes.Buffer)
// 	if _, err := io.Copy(flvBody, video.Data); err != nil {
// 		return err
// 	}
// 	video.Data = flvBody
//
// 	if err := h.flvEnc.Encode(&flvtag.FlvTag{
// 		TagType:   flvtag.TagTypeVideo,
// 		Timestamp: timestamp,
// 		Data:      &video,
// 	}); err != nil {
// 		log.Printf("Failed to write video: Err = %+v", err)
// 	}
//
// 	return nil
// }
//
// func (h *Handler) OnClose() {
// 	log.Printf("OnClose")
//
// 	if h.flvFile != nil {
// 		_ = h.flvFile.Close()
// 	}
// }
