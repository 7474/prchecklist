// Tell React that tests run inside act() so that updates are flushed there.
// https://react.dev/reference/react/act#setting-up-your-test-environment
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
