// Пакет реализации RPC сервера.
package servicerpc

import (
	"context"

	"github.com/Part001-R/YaPrShortener/internal/handler"
	pb "github.com/Part001-R/YaPrShortener/proto/service"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ShortenerService, RPC сервер.
type ShortenerService struct {
	pb.UnimplementedShortenerServiceServer
	Conf *handler.ShortLong
}

// Представление пары сокращения и оригинального URL.
type shortURLOriginalURL struct {
	shortURL    string
	originalURL string
}

// ShortenURL, обработчик. Принимает в запросе длинное представление URL и формирует сокращённое представление. Возвращает короткое представление и ошибку.
// В метаданных, ожидается присутствие данных authorization.
//
// /api/shorten
func (s *ShortenerService) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (res *pb.URLShortenResponse, err error) {

	// Проверка, что сервис в состоянии остановки.
	if s.Conf.IsFlagStopping() {
		return nil, status.Error(codes.Unavailable, serverUnavalible)
	}

	// Приём данных запроса.
	rxLongURL, uuid, err := ShortenURLLayerRx(ctx, req)
	if err != nil {
		switch err {
		case errMissingMetadata:
			return nil, status.Error(codes.Unavailable, missingMetadata)
		case errMissingAuthorization:
			return nil, status.Error(codes.Unavailable, missingAuthorization)
		default:
			s.Conf.Log.Error("RPC. Код ошибки не опознан",
				zap.String("обработчик", "ShortenURL"),
				zap.String("функция", "ShortenURLLayerRx"),
				zap.String("ошибка", err.Error()),
			)
			return nil, status.Error(codes.Internal, internalServerErr)
		}
	}

	// Логика.
	var rxData handler.RxLongURL
	rxData.URL = rxLongURL

	short, flagConflict, err := handler.InternalShortURLFromLongJSONLayerWork(s.Conf, rxData, uuid)
	if err != nil {
		switch err.Error() {
		case "500":
			return nil, status.Error(codes.Internal, internalServerErr)
		default:
			s.Conf.Log.Error("RPC. Код ошибки не опознан",
				zap.String("обработчик", "ShortenURL"),
				zap.String("функция", "handler.InternalShortURLFromLongJSONLayerWork"),
				zap.String("ошибка", err.Error()),
			)
			return nil, status.Error(codes.Internal, internalServerErr)
		}
	}

	// Ответ.
	res, err = ShortenURLLayerTx(short, flagConflict)
	if err != nil {
		s.Conf.Log.Error("RPC. Неожиданная ошибка.",
			zap.String("обработчик", "ShortenURL"),
			zap.String("функция", "ShortenURLLayerTx"),
			zap.String("ошибка", err.Error()),
		)
		return nil, status.Error(codes.Internal, internalServerErr)
	}

	return res, nil
}

// ExpandURL, обработчик. Принимает в запросе URL с сокращнным представлением. Возвращает длинное представление URL и ошибку.
//
// /<id>
func (s *ShortenerService) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (res *pb.URLExpandResponse, err error) {

	// Проверка, что сервис в состоянии остановки.
	if s.Conf.IsFlagStopping() {
		return nil, status.Error(codes.Unavailable, serverUnavalible)
	}

	// Приём данных запроса.
	idRx, err := ExpandURLLayerRx(req)
	if err != nil {
		s.Conf.Log.Error("RPC. Ошибка обработчика.",
			zap.String("обработчик", "ExpandURL"),
			zap.String("функция", "ExpandURLLayerRx"),
			zap.String("ошибка", err.Error()),
		)
		return nil, status.Error(codes.Internal, internalServerErr)
	}

	// Логика.
	longURL, err := handler.InternalLongURLFromShortLayerWork(s.Conf, idRx)
	if err != nil {
		switch err.Error() {
		case "500":
			return nil, status.Error(codes.Internal, internalServerErr)
		case "400":
			return nil, status.Error(codes.InvalidArgument, badRequest)
		case "404":
			return nil, status.Error(codes.Unavailable, dataUnavalible)
		case "410":
			return nil, status.Error(codes.Unavailable, dataUnavalible)
		default:
			s.Conf.Log.Error("RPC. Неопазнанная ошибка.",
				zap.String("обработчик", "ExpandURL"),
				zap.String("функция", "handler.InternalLongURLFromShortLayerWork"),
				zap.String("ошибка", err.Error()),
			)
			return nil, status.Error(codes.Internal, internalServerErr)
		}
	}

	// Ответ.
	res, err = ExpandURLLayerTx(longURL)
	if err != nil {
		s.Conf.Log.Error("RPC. Неожиданная ошибка.",
			zap.String("обработчик", "ExpandURL"),
			zap.String("функция", "ExpandURLLayerTx"),
			zap.String("ошибка", err.Error()),
		)
		return nil, status.Error(codes.Internal, internalServerErr)
	}

	return res, nil
}

// ListUserURLs, обработчик. Возврат списка пар соответствий и ошибку.
//
// /api/user/urls
func (s *ShortenerService) ListUserURLs(ctx context.Context, req *emptypb.Empty) (*pb.UserURLsResponse, error) {

	// Проверка, что сервис в состоянии остановки.
	if s.Conf.IsFlagStopping() {
		return nil, status.Error(codes.Unavailable, serverUnavalible)
	}

	// Логика.
	shortLong, err := handler.InternalUserURLsLayerWork(s.Conf)
	if err != nil {
		switch err.Error() {
		case "500":
			return nil, status.Error(codes.Internal, internalServerErr)
		default:
			s.Conf.Log.Error("RPC. Неопознанная ошибка обработчика.",
				zap.String("обработчик", "ListUserURLs"),
				zap.String("функция", "handler.InternalUserURLsLayerWork"),
				zap.String("ошибка", err.Error()),
			)
			return nil, status.Error(codes.Internal, internalServerErr)
		}
	}

	// Подготовка для отправки.
	list := make([]shortURLOriginalURL, 0)

	for _, v := range shortLong {
		var el shortURLOriginalURL
		el.originalURL = v.OriginalURL
		el.shortURL = v.ShortURL

		list = append(list, el)
	}

	// Ответ.
	res, err := ListUserURLsLayerTx(list)
	if err != nil {
		s.Conf.Log.Error("RPC. Неожиданная ошибка.",
			zap.String("обработчик", "ListUserURLs"),
			zap.String("функция", "ListUserURLsLayerTx"),
			zap.String("ошибка", err.Error()),
		)
		return nil, status.Error(codes.Internal, internalServerErr)
	}

	return res, nil
}
