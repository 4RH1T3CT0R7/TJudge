import { Component } from 'react';
import type { ReactNode, ErrorInfo } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  errorMessage: string;
}

// ErrorBoundary перехватывает непойманные исключения render-tree и отображает
// fallback-UI вместо белого экрана. P1.10: показываем конкретное сообщение
// вместо generic "что-то пошло не так", кнопка "попробовать снова"
// без полного page reload (сохраняет кэш и сессию).
export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, errorMessage: '' };
  }

  static getDerivedStateFromError(error: Error): State {
    return {
      hasError: true,
      errorMessage: error?.message || 'Неизвестная ошибка интерфейса',
    };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // В продакшне полезно отправлять в Sentry/посадочную систему.
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  private handleRetry = () => {
    this.setState({ hasError: false, errorMessage: '' });
  };

  private handleHome = () => {
    this.setState({ hasError: false, errorMessage: '' });
    window.location.href = '/';
  };

  render() {
    if (this.state.hasError) {
      return (
        <div
          className="flex flex-col items-center justify-center min-h-screen gap-4 text-gray-300 p-6"
          role="alert"
          aria-live="assertive"
        >
          <h1 className="text-2xl font-bold">Что-то пошло не так</h1>
          <p className="text-gray-400 text-center max-w-md">
            {this.state.errorMessage}
          </p>
          <div className="flex gap-3">
            <button
              className="px-4 py-2 bg-blue-600 rounded hover:bg-blue-700 transition-colors"
              onClick={this.handleRetry}
              aria-label="Попробовать снова"
            >
              Попробовать снова
            </button>
            <button
              className="px-4 py-2 bg-gray-700 rounded hover:bg-gray-600 transition-colors"
              onClick={this.handleHome}
              aria-label="Вернуться на главную"
            >
              На главную
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
