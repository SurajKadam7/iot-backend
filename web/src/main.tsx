import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { APP_NAME } from "./brand";
import { AppearanceProvider } from "./theme";
import "./styles.css";

if (APP_NAME) {
  document.title = APP_NAME;
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AppearanceProvider>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </AppearanceProvider>
  </React.StrictMode>,
);
