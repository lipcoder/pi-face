package recognition

import (
	"context"
	"fmt"
	"lipcoder/face/internal/adapter/camera"
	"lipcoder/face/internal/adapter/camera/local"
	"net/http"
)

type AdminOp string

const (
	AdminAddFace    AdminOp = "add"
	AdminDeleteFace AdminOp = "delete"
	AdminQueryFace  AdminOp = "query"
)

type AdminRequest struct {
	Op   AdminOp
	Name string

	// add 的时候需要 Cam。
	// delete/query 不需要 Cam。
	Cam camera.Camera

	Reply chan AdminResult
}

type AdminResult struct {
	Op     AdminOp
	Name   string
	ID     int64
	Exists bool
	Err    error
}

// StartAdminLoop 处理管理请求。
// add 会启动新的 goroutine，不阻塞管理循环。
// delete/query 直接按 name 操作数据库。
func (s *FaceVerifyService) StartAdminLoop(
	ctx context.Context,
	reqCh <-chan AdminRequest,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case req, ok := <-reqCh:
			if !ok {
				return nil
			}

			switch req.Op {
			case AdminAddFace:
				go s.handleAddFaceRequest(ctx, req)

			case AdminDeleteFace:
				err := s.DeleteFaceByName(ctx, req.Name)
				sendAdminResult(ctx, req.Reply, AdminResult{
					Op:   req.Op,
					Name: req.Name,
					Err:  err,
				})

			case AdminQueryFace:
				exists, err := s.QueryFaceByName(ctx, req.Name)
				sendAdminResult(ctx, req.Reply, AdminResult{
					Op:     req.Op,
					Name:   req.Name,
					Exists: exists,
					Err:    err,
				})

			default:
				sendAdminResult(ctx, req.Reply, AdminResult{
					Op:   req.Op,
					Name: req.Name,
					Err:  fmt.Errorf("unknown admin op: %s", req.Op),
				})
			}
		}
	}
}

func (s *FaceVerifyService) handleAddFaceRequest(
	ctx context.Context,
	req AdminRequest,
) {
	select {
	case <-ctx.Done():
		sendAdminResult(ctx, req.Reply, AdminResult{
			Op:   req.Op,
			Name: req.Name,
			Err:  ctx.Err(),
		})
		return

	case s.addFaceSem <- struct{}{}:
		defer func() {
			<-s.addFaceSem
		}()
	}

	id, err := s.AddFaceFromCamera(ctx, req.Name, req.Cam)

	sendAdminResult(ctx, req.Reply, AdminResult{
		Op:   req.Op,
		Name: req.Name,
		ID:   id,
		Err:  err,
	})
}

func sendAdminResult(
	ctx context.Context,
	reply chan AdminResult,
	result AdminResult,
) {
	if reply == nil {
		return
	}

	select {
	case <-ctx.Done():
		return

	case reply <- result:
		return
	}
}

// 保留你原来的函数名，避免其他地方已经调用 GetLocalEmbedding 时直接炸。
// 新代码里更建议用 ExtractBestEmbeddingFromCamera(ctx, cam)。
func GetLocalEmbedding(httpClient *http.Client) ([]float64, error) {
	service := NewFaceVerifyService(httpClient)

	return service.ExtractBestEmbeddingFromCamera(
		context.Background(),
		&local.Local{},
	)
}
