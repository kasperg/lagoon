import React from 'react';
import { PageDeployments as Deployments } from '../deployments';

export default {
  component: Deployments,
  title: 'Pages/Deployments',
}

export const Default = () => (
  <p>@TODO: Doesn't work yet. subscribeToMore() causes browser to freeze.</p>
  // <Deployments
  //   router={{
  //     query: {
  //       openshiftProjectName: 'Example',
  //     },
  //   }}
  // />
);
