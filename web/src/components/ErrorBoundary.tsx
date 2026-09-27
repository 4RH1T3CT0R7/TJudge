import { Component } from 'react';
import type { ReactNode, ErrorInfo } from 'react';
import { ErrorState } from './ui/ErrorState';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  errorMessage: string;
}

// ErrorBoundary перехватывает непойманные исключения render-tree и отображает
// fallback-UI вместо белого экрана. Показываем конкретное сообщение
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
        <div className="flex flex-col items-center justify-center min-h-screen p-6">
          <h1 className="text-2xl font-bold text-gray-100">Что-то пошло не так</h1>
          <ErrorState message={this.state.errorMessage} onRetry={this.handleRetry}>
            <button className="btn btn-secondary" onClick={this.handleHome}>
              На главную
            </button>
          </ErrorState>
        </div>
      );
    }

    return this.props.children;
  }
}
