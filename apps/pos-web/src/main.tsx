import "@restpos/ui/tokens.css";
import "@restpos/ui/components.css";
import "./styles/pos.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { SesionProvider } from "./api/sesion";
import { App } from "./App";
import { AtajosProvider } from "./components/atajos";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <SesionProvider>
      <AtajosProvider>
        <App />
      </AtajosProvider>
    </SesionProvider>
  </StrictMode>,
);
