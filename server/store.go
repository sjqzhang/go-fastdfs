package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	log "github.com/sjqzhang/seelog"
	"github.com/busyfree/tusd/pkg/handler"
)

type hookDataStore struct {
	handler.DataStore
}

func (store hookDataStore) NewUpload(ctx context.Context, info handler.FileInfo) (upload handler.Upload, err error) {
	var (
		jsonResult JsonResult
	)
	if Config().AuthUrl != "" {
		if auth_token, ok := info.MetaData["auth_token"]; !ok {
			msg := "token auth fail,auth_token is not in http header Upload-Metadata," +
				"in uppy uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca' })"
			log.Error(msg, fmt.Sprintf("current header:%v", info.MetaData))
			return nil, httpError{error: errors.New(msg), statusCode: 401}
		} else {
			req := httplib.Post(Config().AuthUrl)
			req.Param("auth_token", auth_token)
			req.SetTimeout(time.Second*5, time.Second*10)
			content, err := req.String()
			content = strings.TrimSpace(content)
			if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
				if err = json.Unmarshal([]byte(content), &jsonResult); err != nil {
					log.Error(err)
					return nil, httpError{error: errors.New(err.Error() + content), statusCode: 401}
				}
				if jsonResult.Data != "ok" {
					return nil, httpError{error: errors.New(content), statusCode: 401}
				}
			} else {
				if err != nil {
					log.Error(err)
					return nil, err
				}
				if strings.TrimSpace(content) != "ok" {
					return nil, httpError{error: errors.New(content), statusCode: 401}
				}
			}
		}
	}
	return store.DataStore.NewUpload(ctx, info)
}

//
//func (store hookDataStore) NewUpload(info tusd.FileInfo) (id string, err error) {
//	var (
//		jsonResult JsonResult
//	)
//	if Config().AuthUrl != "" {
//		if auth_token, ok := info.MetaData["auth_token"]; !ok {
//			msg := "token auth fail,auth_token is not in http header Upload-Metadata," +
//				"in uppy uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca' })"
//			log.Error(msg, fmt.Sprintf("current header:%v", info.MetaData))
//			return "", httpError{error: errors.New(msg), statusCode: 401}
//		} else {
//			req := httplib.Post(Config().AuthUrl)
//			req.Param("auth_token", auth_token)
//			req.SetTimeout(time.Second*5, time.Second*10)
//			content, err := req.String()
//			content = strings.TrimSpace(content)
//			if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
//				if err = json.Unmarshal([]byte(content), &jsonResult); err != nil {
//					log.Error(err)
//					return "", httpError{error: errors.New(err.Error() + content), statusCode: 401}
//				}
//				if jsonResult.Data != "ok" {
//					return "", httpError{error: errors.New(content), statusCode: 401}
//				}
//			} else {
//				if err != nil {
//					log.Error(err)
//					return "", err
//				}
//				if strings.TrimSpace(content) != "ok" {
//					return "", httpError{error: errors.New(content), statusCode: 401}
//				}
//			}
//		}
//	}
//	return store.DataStore.NewUpload(info)
//}
