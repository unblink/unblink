import { render } from "solid-js/web";
import App from "./App";
import "./index.css";
import "./ark/ark.css";
import type { JSX } from "solid-js";

// Store the dispose function for HMR
let dispose: (() => void) | undefined;

const renderApp = () => {
    const appElement = document.getElementById("root")!;

    // Clear the root element first to ensure no duplicate content
    appElement.innerHTML = "";

    // Dispose of previous render if exists
    if (dispose) {
        dispose();
        dispose = undefined;
    }

    // Render the app and store the dispose function
    dispose = render(() => <App /> as JSX.Element, appElement);
};

if (import.meta.hot) {
    // Accept updates for this module
    import.meta.hot.accept(() => {
        console.log("HMR update received!");
        // Re-render the app with the updated module
        renderApp();
    });

    // Dispose when this module is about to be replaced
    import.meta.hot.dispose(() => {
        if (dispose) {
            dispose();
            dispose = undefined;
        }
    });
}

// Initial render
renderApp();
