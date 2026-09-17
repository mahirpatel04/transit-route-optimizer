import Header from './components/Header';
import RoutesButton from './components/RoutesButton';
import StopsNearForm from './components/StopsNearForm';
import './App.css';

export default function App() {
  return (
    <div className="app">
      <Header />
      <main>
        <h1>NYC Transit</h1>
        <RoutesButton />
        <StopsNearForm />
      </main>
    </div>
  );
}
