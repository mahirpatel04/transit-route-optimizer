import Header from './components/Header';
import RouteFinder from './components/RouteFinder';
import './App.css';

export default function App() {
  return (
    <div className="app">
      <Header />
      <main>
        <h1>NYC Transit</h1>
        <RouteFinder />
      </main>
      <footer className="app-footer">
        REPO:{' '}
        <a href="https://github.com/mahirpatel04/transit-route-optimizer" target="_blank" rel="noopener noreferrer">
          github.com/mahirpatel04/transit-route-optimizer
        </a>
      </footer>
    </div>
  );
}
