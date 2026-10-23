import './style.css';
import { mountAuth } from './auth/shell.js';

const app = document.querySelector<HTMLElement>('#app');
if (app) {
  const unmount = mountAuth(app);
  if (import.meta.hot) import.meta.hot.dispose(unmount);
}
