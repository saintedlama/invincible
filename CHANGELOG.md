# Changelog

## [1.6.1](https://github.com/saintedlama/invincible/compare/v1.6.0...v1.6.1) (2026-06-15)


### Bug Fixes

* split skill and skill-spec commands for simpler agent install ([e0e2a2d](https://github.com/saintedlama/invincible/commit/e0e2a2d741030104b7c476cd26ec698a2c054e13))

## [1.6.0](https://github.com/saintedlama/invincible/compare/v1.5.1...v1.6.0) (2026-06-13)


### Features

* add l and p keyboard shortcuts for quick screen switching ([57644a6](https://github.com/saintedlama/invincible/commit/57644a637dd372de12d11b9516f829c0bc1c6a9e))


### Bug Fixes

* add install or update skill ([8644c90](https://github.com/saintedlama/invincible/commit/8644c90ac1e2b3f2b8529dda6f06e77da15c866c))
* use mouse wheel for scrolling in the processes and logs ([5b8e93f](https://github.com/saintedlama/invincible/commit/5b8e93f23cfcad2783a9707ea7df278d1104c849))

## [1.5.1](https://github.com/saintedlama/invincible/compare/v1.5.0...v1.5.1) (2026-06-13)


### Bug Fixes

* emit a project agnostic skill ([5ef68df](https://github.com/saintedlama/invincible/commit/5ef68df89a0fc06bc1ffb2ab06bf99591c181f35))

## [1.5.0](https://github.com/saintedlama/invincible/compare/v1.4.0...v1.5.0) (2026-06-13)


### Features

* add overview information to the process list ([51da0a7](https://github.com/saintedlama/invincible/commit/51da0a7af43eae10f437f5b4d2f16a3b197b756c))
* reworked layout of the tui to support smaller screens ([be029c4](https://github.com/saintedlama/invincible/commit/be029c42153ccadff25ae136305b0ed913f8cbc8))
* support multiple instances with pid files ([cd0aa94](https://github.com/saintedlama/invincible/commit/cd0aa9423ed1b02f18cce6cbab3cd540d406c2d5))


### Bug Fixes

* update skill and init commands to match features ([c4ecee9](https://github.com/saintedlama/invincible/commit/c4ecee94c1c2c6664787b6d07327df42a281dea7))

## [1.4.0](https://github.com/saintedlama/invincible/compare/v1.3.0...v1.4.0) (2026-06-10)


### Features

* add stop/start/restart all ([32dae26](https://github.com/saintedlama/invincible/commit/32dae2648aad539acc512955143905d9e73f5224))
* add watch and build support, fix stopping of processes, faster ([27a8bc2](https://github.com/saintedlama/invincible/commit/27a8bc22cd4a2ed807f3aa2473b3519604c50e72))
* use dependency graph for ordered start/stop, add building state ([da30d91](https://github.com/saintedlama/invincible/commit/da30d919618efd5a7c9d3717fbe0a2853a1c6e1e))


### Bug Fixes

* fix shutdown period, condense ui for smaller screens, fix mouse ([57be4ae](https://github.com/saintedlama/invincible/commit/57be4ae307cfa5ca6067d3e36eccff8959014572))

## [1.3.0](https://github.com/saintedlama/invincible/compare/v1.2.0...v1.3.0) (2026-06-07)


### Features

* display process status in window title ([3923621](https://github.com/saintedlama/invincible/commit/3923621d9b786f25207067bd015090056a6dca8c))

## [1.2.0](https://github.com/saintedlama/invincible/compare/v1.1.0...v1.2.0) (2026-06-06)


### Features

* add cwd option to set working directory per process ([3bdfa04](https://github.com/saintedlama/invincible/commit/3bdfa04aa03a7bea42d15ccdfab425de07b58f17))


### Bug Fixes

* use cmd on win if no sh installed ([4ee4be2](https://github.com/saintedlama/invincible/commit/4ee4be2261ce06bc085c811cfc35f4ad2c063fec))

## [1.1.0](https://github.com/saintedlama/invincible/compare/v1.0.0...v1.1.0) (2026-06-04)


### Features

* graceful shutdown ([04dbea9](https://github.com/saintedlama/invincible/commit/04dbea9a6e6722e60a569a19dd21b19ae45f9e1e))


### Bug Fixes

* init now outputs to a file ([b5f2495](https://github.com/saintedlama/invincible/commit/b5f2495241339e68685ee37c4e5cd5539ff76c9c))
* use .invincible.toml for config by default ([72dc32e](https://github.com/saintedlama/invincible/commit/72dc32e22a4ebaa06f19c74318cfdd203b3a4e58))

## 1.0.0 (2026-06-04)


### Features

* dependency aware startup ([d0c701c](https://github.com/saintedlama/invincible/commit/d0c701c40853bd6907959eafe3a1ba1dfeb8915a))
