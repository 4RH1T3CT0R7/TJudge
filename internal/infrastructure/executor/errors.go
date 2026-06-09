package executor

import (
	stderrors "errors"
	"fmt"
)

// InfraError помечает инфраструктурные (транзиентные) ошибки исполнения матча:
// проблема не в программе участника, а в окружении - Docker daemon недоступен,
// образ отсутствует, контейнер не создался. Такие матчи нельзя помечать failed:
// программа не виновата, матч безопасно вернуть в pending и повторить.
//
// Ошибки самой программы (ненулевой exit-code, мусорный вывод, превышение
// таймаута исполнения) InfraError НЕ являются - они терминальны.
type InfraError struct {
	Err error
}

func (e *InfraError) Error() string { return e.Err.Error() }
func (e *InfraError) Unwrap() error { return e.Err }

// infraErrorf оборачивает ошибку как инфраструктурную.
func infraErrorf(format string, args ...any) error {
	return &InfraError{Err: fmt.Errorf(format, args...)}
}

// IsInfraError сообщает, является ли ошибка инфраструктурной (транзиентной).
func IsInfraError(err error) bool {
	var ie *InfraError
	return stderrors.As(err, &ie)
}
