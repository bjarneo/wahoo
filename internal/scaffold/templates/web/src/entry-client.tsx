import { StrictMode, startTransition } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import App from "./App";
import "./styles.css";

startTransition(() => {
  const root = document.getElementById("root");
  if (!root) {
    return;
  }
  const app = <StrictMode><App /></StrictMode>;
  if (root.hasChildNodes()) {
    hydrateRoot(root, app);
  } else {
    createRoot(root).render(app);
  }
});
