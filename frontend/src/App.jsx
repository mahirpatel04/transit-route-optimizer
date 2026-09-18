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
    </div>
  );
}
