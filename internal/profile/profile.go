// profile пакет, содержит функции для реализации профилирования.
//
// CPU - профилирование CPU.
// Memory - профилирование Memory.
package profile

import (
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

const (
	cpuProfileName = `cpu.profile`
	memProfileName = `mem.profile`
)

// Сбор профиля CPU.
func CPU() (fileCPU func() error, pprofCPU func()) {

	// Файл для записи данных.
	fcpu, err := os.Create(cpuProfileName)
	if err != nil {
		panic(err)
	}
	fileCPU = func() error {
		return fcpu.Close()
	}

	// Сбор профиля.
	if err := pprof.StartCPUProfile(fcpu); err != nil {
		if ferr := fcpu.Close(); ferr != nil {
			panic(err)
		}
		panic(err)
	}
	pprofCPU = func() {
		pprof.StopCPUProfile()
	}

	log.Println("Профилирование CPU запущено")
	return
}

// Сбор профиля памяти.
func Memory() (fileMem func() error) {

	// Файл для записи данных.
	fmem, err := os.Create(memProfileName)
	if err != nil {
		panic(err)
	}
	fileMem = func() error {
		return fmem.Close()
	}

	// Сбор профиля.
	runtime.GC()
	if err := pprof.WriteHeapProfile(fmem); err != nil {
		panic(err)
	}

	log.Println("Профилирование памяти запущено")
	return
}
