import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import './styles/tokens.css';
import './styles/base.css';
import './styles/controls.css';
import './styles/layout.css';
import './styles/board.css';
import './styles/devagotchi.css';
import './styles/gate.css';
import './styles/actions.css';

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>);
