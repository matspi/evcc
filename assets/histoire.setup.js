import { defineSetupVue3 } from "@histoire/plugin-vue";
import "bootstrap/dist/css/bootstrap.min.css";
import "bootstrap";
import smoothscroll from "smoothscroll-polyfill";
import VueNumber from "vue-number-animation";
import i18n from "./js/i18n";
import "./css/app.css";
import { watchThemeChanges } from "./js/theme";

smoothscroll.polyfill();
watchThemeChanges();

export const setupVue3 = defineSetupVue3(({ app }) => {
  app.use(i18n);
  app.use(VueNumber);
});
