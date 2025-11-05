# Профилирование функции internalDeleteUserURLs, слоя internalDeleteUserURLsLayerRx

Профилирование реализовано в тесте `Benchmark_InternalDeleteUserURLs_SUCCESS`

## Сегмент до оптимизации

func internalDeleteUserURLsLayerRx(r *http.Request) (rxArr []string, uuidRx string, err error) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("в аргументе r нет указателя")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	uuidRx = r.Header.Get("Authorization")

	rxByteBody, err := io.ReadAll(r.Body)   // --------------- Будет оптимизировано
	if err != nil {
		logger.Log.Error("Ошибка чтения тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

  ......

  ## Сегмент после оптимизации

  func internalDeleteUserURLsLayerRx(r *http.Request, logger *zap.Logger) (rxArr []string, uuidRx string, err error) {

	// Проверка аргументов.
	if logger == nil {
		log.Println("В аргументе logger, функции internalDeleteUserURLsLayerRx, нет указателя")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	uuidRx = r.Header.Get("Authorization")

	// Применение json.Decoder для оптимизации.  
	decoder := json.NewDecoder(r.Body)                  // ------------- Выполнена оптимизация. Убрана лишняя аллокация памяти. 
	if err := decoder.Decode(&rxArr); err != nil {
		logger.Error("Ошибка сериализации данных",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", fmt.Errorf("%d", http.StatusBadRequest)
	}
  ...

# Данные профилирования

## До оптимизации

(pprof) top
Showing nodes accounting for 4107.77kB, 100% of 4107.77kB total
Showing top 10 nodes out of 14
      flat  flat%   sum%        cum   cum%
    3591kB 87.42% 87.42%     3591kB 87.42%  runtime.allocm
  516.76kB 12.58%   100%   516.76kB 12.58%  runtime.procresize
         0     0%   100%     3078kB 74.93%  runtime.mcall
         0     0%   100%      513kB 12.49%  runtime.mstart
         0     0%   100%      513kB 12.49%  runtime.mstart0
         0     0%   100%      513kB 12.49%  runtime.mstart1
         0     0%   100%     3591kB 87.42%  runtime.newm
         0     0%   100%     3078kB 74.93%  runtime.park_m
         0     0%   100%     3591kB 87.42%  runtime.resetspinning
         0     0%   100%   516.76kB 12.58%  runtime.rt0_go

## После оптимизации

top
Showing nodes accounting for 3594.77kB, 100% of 3594.77kB total
Showing top 10 nodes out of 14
      flat  flat%   sum%        cum   cum%
    3078kB 85.62% 85.62%     3078kB 85.62%  runtime.allocm
  516.76kB 14.38%   100%   516.76kB 14.38%  runtime.procresize
         0     0%   100%     2565kB 71.35%  runtime.mcall
         0     0%   100%      513kB 14.27%  runtime.mstart
         0     0%   100%      513kB 14.27%  runtime.mstart0
         0     0%   100%      513kB 14.27%  runtime.mstart1
         0     0%   100%     3078kB 85.62%  runtime.newm
         0     0%   100%     2565kB 71.35%  runtime.park_m
         0     0%   100%     3078kB 85.62%  runtime.resetspinning
         0     0%   100%   516.76kB 14.38%  runtime.rt0_go

## Результат опитимазиции

File: handler.test
Build ID: 4c709772d8dd9c91e861fd62176cb4c53e18b9fa
Type: inuse_space
Time: 2025-10-28 07:31:32 +07
Showing nodes accounting for -513kB, 12.49% of 4107.77kB total
      flat  flat%   sum%        cum   cum%
    -513kB 12.49% 12.49%     -513kB 12.49%  runtime.allocm
         0     0% 12.49%     -513kB 12.49%  runtime.mcall
         0     0% 12.49%     -513kB 12.49%  runtime.newm
         0     0% 12.49%     -513kB 12.49%  runtime.park_m
         0     0% 12.49%     -513kB 12.49%  runtime.resetspinning
         0     0% 12.49%     -513kB 12.49%  runtime.schedule
         0     0% 12.49%     -513kB 12.49%  runtime.startm
         0     0% 12.49%     -513kB 12.49%  runtime.wakep