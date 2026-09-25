package worker

import (
	"context"
	"sync"
	"time"

	"github.com/bmstu-itstech/tjudge/internal/cache"
	"github.com/bmstu-itstech/tjudge/internal/events"
	"github.com/bmstu-itstech/tjudge/internal/executor"
	"github.com/bmstu-itstech/tjudge/internal/models"
	"github.com/bmstu-itstech/tjudge/internal/queue"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CompileQueue - очередь задач компиляции
type CompileQueue interface {
	Enqueue(ctx context.Context, programID uuid.UUID) error
	Dequeue(ctx context.Context, timeout time.Duration) (*queue.CompileTask, error)
}

// ProgramCompiler собирает программу в докер-песочнице
type ProgramCompiler interface {
	Compile(ctx context.Context, program *models.Program) (*executor.CompileResult, error)
}

// compileLockTTL - ttl лока сборки. WithLock продлевает его, пока сборка идёт,
// а у убитого воркера лок протухает и программу можно собрать снова
const compileLockTTL = 30 * time.Second

func compileLockKey(programID uuid.UUID) string {
	return "compile:" + programID.String()
}

// CompileWorker разбирает очередь компиляции: задача -> сборка в песочнице ->
// статус compiling -> ready/failed.
// заодно периодически возвращает в очередь программы, зависшие в compiling
// (задача потерялась: апи упал между созданием и enqueue, редис моргнул,
// воркер перезапустился посреди сборки)
type CompileWorker struct {
	queue       CompileQueue
	programRepo ProgramRepository
	compiler    ProgramCompiler
	lock        *cache.DistributedLock
	notifier    events.Notifier
	log         *logger.Logger

	workers          int
	stuckInterval    time.Duration
	stuckOlderThan   time.Duration
	stuckBatchSize   int
	dequeueWaitLimit time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCompileWorker создаёт обработчик очереди компиляции
func NewCompileWorker(
	q CompileQueue,
	programRepo ProgramRepository,
	compiler ProgramCompiler,
	lock *cache.DistributedLock,
	notifier events.Notifier,
	log *logger.Logger,
	workers int,
) *CompileWorker {
	return &CompileWorker{
		queue:            q,
		programRepo:      programRepo,
		compiler:         compiler,
		lock:             lock,
		notifier:         notifier,
		log:              log,
		workers:          workers,
		stuckInterval:    60 * time.Second,
		stuckOlderThan:   2 * time.Minute,
		stuckBatchSize:   100,
		dequeueWaitLimit: 2 * time.Second,
	}
}

// Start поднимает воркеры компиляции и recovery-горутину
func (w *CompileWorker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go func(id int) {
			defer w.wg.Done()
			w.runWorker(ctx, id)
		}(i + 1)
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.runStuckRecovery(ctx)
	}()

	w.log.Info("Compile worker started", zap.Int("workers", w.workers))
}

// Stop гасит воркеры, текущие задачи дозакончатся
func (w *CompileWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
		w.wg.Wait()
	}
}

func (w *CompileWorker) runWorker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := w.queue.Dequeue(ctx, w.dequeueWaitLimit)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.log.LogError("Compile queue dequeue failed", err)
			// пауза чтобы не крутить hot-loop когда редис недоступен
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}
		if task == nil {
			continue // таймаут - очередь пуста
		}

		w.processTask(ctx, id, task)
	}
}

// processTask собирает программу под локом: одну программу не собирают
// параллельно (дубль задачи, соседняя реплика). дубль не ждёт - идущая сборка
// сама запишет итог, а статус внутри лока перечитывается, так что поздняя
// копия не перезапишет его
func (w *CompileWorker) processTask(ctx context.Context, workerID int, task *queue.CompileTask) {
	err := w.lock.WithLock(ctx, compileLockKey(task.ProgramID), compileLockTTL, func(ctx context.Context) error {
		w.compile(ctx, workerID, task)
		return nil
	})
	if err != nil {
		w.log.Info("Compile task skipped: program is already being compiled",
			zap.String("program_id", task.ProgramID.String()), zap.Error(err))
	}
}

func (w *CompileWorker) compile(ctx context.Context, workerID int, task *queue.CompileTask) {
	program, err := w.programRepo.GetByID(ctx, task.ProgramID)
	if err != nil {
		if isNotFoundError(err) {
			w.log.Info("Compile task skipped: program deleted",
				zap.String("program_id", task.ProgramID.String()))
			return
		}
		w.log.LogError("Compile task: failed to load program", err,
			zap.String("program_id", task.ProgramID.String()))
		return // stuck-recovery вернёт программу в очередь
	}

	// дубль задачи (stuck-recovery + оригинал) - программа уже обработана
	if program.Status != models.ProgramCompiling {
		return
	}

	w.log.Info("Compiling program",
		zap.Int("worker_id", workerID),
		zap.String("program_id", program.ID.String()),
		zap.String("language", program.Language),
	)

	result, err := w.compiler.Compile(ctx, program)
	if err != nil {
		// инфра-ошибка (докер недоступен, образа нет): программа остаётся
		// в compiling, stuck-recovery повторит позже. в failed переводить
		// нельзя - код тут ни при чём
		w.log.LogError("Compile failed with infra error, will retry", err,
			zap.String("program_id", program.ID.String()))
		return
	}

	status := models.ProgramReady
	codePath := result.ExecPath
	var errMsg *string
	if !result.OK {
		status = models.ProgramFailed
		codePath = program.CodePath
		errMsg = &result.Log
	}

	if err := w.programRepo.UpdateCompileResult(ctx, program.ID, status, codePath, errMsg); err != nil {
		w.log.LogError("Failed to save compile result", err,
			zap.String("program_id", program.ID.String()))
		return
	}

	w.publishCompiled(ctx, program, status, errMsg)

	w.log.Info("Program compiled",
		zap.Int("worker_id", workerID),
		zap.String("program_id", program.ID.String()),
		zap.String("status", string(status)),
	)
}

// publishCompiled шлёт событие ProgramCompiled, best-effort
func (w *CompileWorker) publishCompiled(ctx context.Context, program *models.Program, status models.ProgramStatus, errMsg *string) {
	evt := events.ProgramCompiled{
		Version:      1,
		ProgramID:    program.ID,
		Status:       string(status),
		ErrorMessage: errMsg,
	}
	if program.TournamentID != nil {
		evt.TournamentID = *program.TournamentID
	}
	if program.TeamID != nil {
		evt.TeamID = *program.TeamID
	}
	w.notifier.ProgramCompiled(ctx, evt)
}

// runStuckRecovery раз в минуту перезакидывает зависшие compiling в очередь
func (w *CompileWorker) runStuckRecovery(ctx context.Context) {
	ticker := time.NewTicker(w.stuckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.recoverStuck(ctx)
		}
	}
}

func (w *CompileWorker) recoverStuck(ctx context.Context) {
	programs, err := w.programRepo.GetStuckCompiling(ctx, w.stuckOlderThan, w.stuckBatchSize)
	if err != nil {
		w.log.LogError("Failed to find stuck compiling programs", err)
		return
	}
	if len(programs) == 0 {
		return
	}

	w.log.Info("Re-enqueueing stuck compiling programs", zap.Int("count", len(programs)))
	for _, p := range programs {
		// идущую сборку не дублировать; задачу, уже лежащую в очереди,
		// отсекает дедуп самой очереди
		if locked, err := w.lock.IsLocked(ctx, compileLockKey(p.ID)); err == nil && locked {
			continue
		}
		if err := w.queue.Enqueue(ctx, p.ID); err != nil {
			w.log.LogError("Failed to re-enqueue stuck program", err,
				zap.String("program_id", p.ID.String()))
		}
	}
}
