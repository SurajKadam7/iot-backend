/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_AUTH_MODE: string;
  readonly VITE_COGNITO_REGION: string;
  readonly VITE_COGNITO_CLIENT_ID: string;
  readonly VITE_APP_NAME: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
