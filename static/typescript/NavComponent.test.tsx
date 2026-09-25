import * as React from "react";
import { act } from "react";
import * as renderer from "react-test-renderer";

import { NavComponent } from "./NavComponent";

test("", async () => {
  let component!: renderer.ReactTestRenderer;
  act(() => {
    component = renderer.create(<NavComponent />);
  });

  const tree = component.toJSON();
  expect(tree).toMatchSnapshot();
});
