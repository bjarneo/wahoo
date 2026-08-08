import { StrictMode, startTransition } from "react";
import { hydrateRoot } from "react-dom/client";
import App from "./App";
import "./styles.css";

startTransition(() => {
  hydrateRoot(
    document.getElementById("root")!,
    <StrictMode><App /></StrictMode>,
  );
});
