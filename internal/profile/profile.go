// profile пакет, содержит функции для реализации профилирования.
//
// CPU - профилирование CPU.
// Memory - профилирование Memory.
package profile

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"go.uber.org/zap"
)

const (
	cpuProfileName = `cpu.profile`
	memProfileName = `mem.profile`
)

// CPU, реализует сбор профиля CPU. Возвращается функция с закрытием подключения к файлу, функция остановки профилирования и ошибка.
//
// Параметры:
//
//	logger - логгер.
func CPU(logger *zap.Logger) (closeFileCPU func() error, stopPprofCPU func(), err error) {

	// Файл для записи данных.
	fcpu, err := os.Create(cpuProfileName)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка создания файла профилирования CPU: <%w>", err)
	}
	closeFileCPU = func() error {
		return fcpu.Close()
	}

	// Сбор профиля.
	if err := pprof.StartCPUProfile(fcpu); err != nil {
		if ferr := fcpu.Close(); ferr != nil {
			return nil, nil, fmt.Errorf("ошибка закрытия подключения к файлу: <%w>, при ошибке запуска профилирования CPU: <%w>", err, ferr)
		}
		return nil, nil, fmt.Errorf("ошибка запуска профилирования CPU: <%w>", err)
	}
	stopPprofCPU = func() {
		pprof.StopCPUProfile()
	}

	logger.Info("Прифилирование CPU запущено")
	return closeFileCPU, stopPprofCPU, nil
}

// Memory, реализует сбор профиля памяти. Возвращается функция с закрытием подключения к файлу и ошибка.
//
// Параметры:
//
//	logger - логгер.
func Memory(logger *zap.Logger) (closeFileMem func() error, err error) {

	// Файл для записи данных.
	fmem, err := os.Create(memProfileName)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания файла профилирования CPU: <%w>", err)
	}
	closeFileMem = func() error {
		return fmem.Close()
	}

	// Сбор профиля.
	runtime.GC()
	if err := pprof.WriteHeapProfile(fmem); err != nil {
		if ferr := fmem.Close(); ferr != nil {
			return nil, fmt.Errorf("ошибка закрытия подключения к файлу: <%w>, при ошибке запуска профилирования памяти: <%w>", err, ferr)
		}
		return nil, fmt.Errorf("ошибка запуска профилирования памяти: <%w>", err)
	}

	logger.Info("Профилирование памяти запущено")
	return closeFileMem, nil
}
