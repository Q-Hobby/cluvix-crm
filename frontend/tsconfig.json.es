/* To learn more about Typescript configuration file: https://www.typescriptlang.org/docs/handbook/tsconfig-json.html. */
/* To learn more about Angular compiler options: https://angular.dev/reference/configs/angular-compiler-options. */
{
  "compileOnSave": false,
  "compilerOptions": {
    "outDir": "./dist/out-tsc",
    // ✅ Strict mode đầy đủ, nên giữ true để giảm bug
    "strict": true,
    "noImplicitAny": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noImplicitOverride": true,
    "strictNullChecks": true,

    // ✅ Bundler là mặc định mới với esbuild
    "moduleResolution": "bundler",
    "target": "ES2022",
    "module": "ESNext",
    "importHelpers": true,
    "isolatedModules": true,
    "esModuleInterop": true,
    "experimentalDecorators": true,
    "skipLibCheck": true,
    "downlevelIteration": true,
    "useDefineForClassFields": true,
    "forceConsistentCasingInFileNames": true,
    // ✅ Source map nên giữ true trong dev
    "sourceMap": true,

    "noPropertyAccessFromIndexSignature": true,

    "baseUrl": "./",
    "paths": {
      "@/*": [
        "src/app/*"
      ],
      "@env/*": [
        "./src/environments/*"
      ],
      "@assets/*": [
        "./src/assets/*"
      ],
    },
    "allowSyntheticDefaultImports": true,
    "lib": [
      "ES2022",
      "dom"
    ],
    "types": []
  },
  "angularCompilerOptions": {
    "enableI18nLegacyMessageIdFormat": false,
    "strictInjectionParameters": true,
    "strictInputAccessModifiers": true,
    "strictTemplates": true
  }
}
