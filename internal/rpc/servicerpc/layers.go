package servicerpc

import (
	"context"

	pb "github.com/Part001-R/YaPrShortener/proto/service"
	"google.golang.org/grpc/metadata"
)

// ShortenURL
//
// Получение данных из запроса.
func ShortenURLLayerRx(ctx context.Context, req *pb.URLShortenRequest) (rxLongURL, uuid string, err error) {

	// Метаданные.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", errMissingMetadata
	}

	uuidRx := md["authorization"]
	if len(uuidRx) == 0 {
		return "", "", errMissingAuthorization
	}

	// Результат
	return req.Url, uuidRx[0], nil
}

// Формирование ответа.
func ShortenURLLayerTx(short string, flagConflict bool) (*pb.URLShortenResponse, error) {

	// Проверка признака конфликта.
	if flagConflict {
		return nil, errAlreadyExixts
	}

	// Ответ.
	return &pb.URLShortenResponse{Result: short}, nil
}

// ExpandURL
//
// Получение данных из запроса.
func ExpandURLLayerRx(req *pb.URLExpandRequest) (string, error) {

	return req.Id, nil
}

// Формирование ответа.
func ExpandURLLayerTx(longURL string) (*pb.URLExpandResponse, error) {

	return &pb.URLExpandResponse{Result: longURL}, nil
}

// ListUserURLs
//
// Формирование ответа.
func ListUserURLsLayerTx(list []shortURLOriginalURL) (*pb.UserURLsResponse, error) {

	var urlDataList []*pb.URLData

	for _, v := range list {
		urlData := &pb.URLData{
			ShortUrl:    v.shortURL,
			OriginalUrl: v.originalURL,
		}
		urlDataList = append(urlDataList, urlData)
	}

	return &pb.UserURLsResponse{Url: urlDataList}, nil
}
