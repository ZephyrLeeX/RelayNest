import './style.css';

const app = document.querySelector<HTMLElement>('#app');
if (app) {
  app.dataset.state = 'ready';
}
