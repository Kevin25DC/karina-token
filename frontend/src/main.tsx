import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';

// Diagnóstico: guarda los errores del frontend en un archivo local para poder
// investigar problemas del webview. No contiene credenciales.
function reportError(message: string, stack: string) {
  try {
    const go = (window as unknown as {
      go?: { main?: { App?: { LogClientError?: (m: string, s: string) => void } } };
    }).go;
    go?.main?.App?.LogClientError?.(message, stack);
  } catch {
    /* ignore */
  }
}

window.addEventListener('error', (e) => {
  reportError(String(e.message), String(e.error?.stack ?? ''));
});
window.addEventListener('unhandledrejection', (e) => {
  const reason = e.reason;
  reportError(
    reason instanceof Error ? reason.message : String(reason),
    reason instanceof Error ? String(reason.stack ?? '') : '',
  );
});

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
