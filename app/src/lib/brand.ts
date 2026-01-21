import { createMemo } from "solid-js";

export const isZapdos = createMemo(() =>
  window.location.hostname.endsWith(".zapdoslabs.com")
);

export const brandName = () => isZapdos() ? "zapdos labs" : "unblink";

export const brandLogo = () => isZapdos() ? "/zapdos_logo.svg" : "/logo.svg";

export const brandFavicon = () => isZapdos() ? "/zapdos_favicon.ico" : "/favicon.ico";
