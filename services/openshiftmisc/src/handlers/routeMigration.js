// @flow

const promisify = require('util').promisify;
const OpenShiftClient = require('openshift-client');
const R = require('ramda');
const { logger } = require('@lagoon/commons/src/local-logging');
const { sendToLagoonLogs } = require('@lagoon/commons/src/logs');
const {
  getOpenShiftInfoForProject,
  getEnvironmentByOpenshiftProjectName,
  updateProject,
  addTask,
  updateTask,
} = require('@lagoon/commons/src/api');
const { RouteMigration } = require('@lagoon/commons/src/openshiftApi');
const uuid4 = require('uuid4');
const convertDateFormat = R.init;

async function routeMigration (data: Object) {
  const { projectName, productionEnvironment, standbyProductionEnvironment } = data;

  const result = await getOpenShiftInfoForProject(projectName);
  const projectOpenShift = result.project;
  const ocsafety = string =>
    string.toLocaleLowerCase().replace(/[^0-9a-z-]/g, '-');

  try {
    var safeActiveProductionEnvironment = ocsafety(productionEnvironment);
    var safeStandbyProductionEnvironment = ocsafety(standbyProductionEnvironment);
    var safeProjectName = ocsafety(projectName);
    var openshiftConsole = projectOpenShift.openshift.consoleUrl.replace(
      /\/$/,
      ''
    );
    var openshiftToken = projectOpenShift.openshift.token || '';
    var openshiftProject = projectOpenShift.openshiftProjectPattern
      ? projectOpenShift.openshiftProjectPattern
          .replace('${branch}', safeActiveProductionEnvironment)
          .replace('${project}', safeProjectName)
      : `${safeProjectName}-${safeActiveProductionEnvironment}`;
    // create the destination openshift project name
    var destinationOpenshiftProject = projectOpenShift.openshiftProjectPattern
      ? projectOpenShift.openshiftProjectPattern
          .replace('${branch}', safeStandbyProductionEnvironment)
          .replace('${project}', safeProjectName)
      : `${safeProjectName}-${safeStandbyProductionEnvironment}`;
  } catch (error) {
    logger.error(`Error while loading information for project ${projectName}`);
    logger.error(error);
    throw error;
  }
  // get the environmentid for the source environment
  const sourceEnvironment = await getEnvironmentByOpenshiftProjectName(openshiftProject);

  // define the routemigration. the annotation being set to true is what actually triggers the switch
  const migrateRoutes = (openshiftProject, destinationOpenshiftProject) => {
    let config = {
      apiVersion: 'dioscuri.amazee.io/v1',
      kind: 'RouteMigrate',
      metadata: {
        name: openshiftProject,
        annotations: {
            'dioscuri.amazee.io/migrate':'true'
        }
      },
      spec: {
        destinationNamespace: destinationOpenshiftProject,
        activeEnvironment: safeActiveProductionEnvironment,
      },
    };

    return config;
  };

  // Kubernetes API Object - needed as some API calls are done to the Kubernetes API part of OpenShift and
  // the OpenShift API does not support them.
  const dioscuri = new RouteMigration({
    url: openshiftConsole,
    insecureSkipTlsVerify: true,
    auth: {
      bearer: openshiftToken
    }
  });
  const openshift = new OpenShiftClient.OApi({
    url: openshiftConsole,
    insecureSkipTlsVerify: true,
    auth: {
      bearer: openshiftToken
    }
  });

  // generate a uuid for this event
  var uuid = uuid4();

  // check that the namespaces exist for source and destination before we try and move any routes
  try {
    // check source
    const projectSourceGet = promisify(openshift.projects(openshiftProject).get, { context: openshift.projects(openshiftProject) })
    projectStatus = await projectSourceGet()
    logger.info(`${openshiftProject}: Project ${openshiftProject} already exists, continuing`)
    // check dest
    const projectDestGet = promisify(openshift.projects(destinationOpenshiftProject).get, { context: openshift.projects(destinationOpenshiftProject) })
    projectStatus = await projectDestGet()
    logger.info(`${openshiftProject}: Project ${destinationOpenshiftProject} already exists, continuing`)
  } catch (err) {
    // throw error if the namespace doesn't exist
    logger.error(err)
    throw new Error
  }

  var sourceTaskID = null
  try {
    // add a task into the environment
    var date = new Date()
    var created = convertDateFormat(date.toISOString())
    const sourceTaskData = await addTask(
      'Active/Standby Switch',
      'ACTIVE',
      created,
      sourceEnvironment.environmentByOpenshiftProjectName.id,
      uuid,
      null,
      null,
      null,
      '',
      '',
      false,
    );
    sourceTaskID = sourceTaskData.addTask.id
  } catch (err) {
    // throw error if the namespace doesn't exist
    logger.error(err)
    throw new Error
  }

  // @TODO: this seems a bit silly, might be a better way to do it. but `.patch` on the routemigrates resource fails with,
  // research says this is because crd is not supported to be patched
  // `message=the body of the request was in an unknown format - accepted media types include: application/json-patch+json`
  try {
    const migrateRoutesDelete = promisify(
      dioscuri.ns(openshiftProject).routemigrates(openshiftProject).delete
    );
    await migrateRoutesDelete({
      body: {}
    });
    await new Promise(resolve => setTimeout(resolve, 10000)); // sleep a bit after deleting
    try {
      const migrateRoutesPost = promisify(
          dioscuri.ns(openshiftProject).routemigrates.post
      );
      await migrateRoutesPost({
        body: migrateRoutes(openshiftProject, destinationOpenshiftProject)
      });
      logger.verbose(`${openshiftProject}: created routeMigration resource`);
    } catch (err) {
        logger.error(err);
        throw new Error();
    }
  } catch (err) {
    await new Promise(resolve => setTimeout(resolve, 1000)); // sleep a bit before creating
    try {
      const migrateRoutesPost = promisify(
          dioscuri.ns(openshiftProject).routemigrates.post
      );
      await migrateRoutesPost({
        body: migrateRoutes(openshiftProject, destinationOpenshiftProject)
      });
      logger.verbose(`${openshiftProject}: created routeMigration resource`);
    } catch (err) {
        logger.error(err);
        throw new Error();
    }
  }

  // check the route migrate resource for the status conditions, only update lagoon on a completed task
  var whileCount = 0;
  var breakLoop = false;
  while (whileCount < 10 && !breakLoop) {
    try {
      const migrateRoutesGet = promisify(
          dioscuri.ns(openshiftProject).routemigrates(openshiftProject).get
      );
      routeMigrateStatus = await migrateRoutesGet();
      try {
        for (i = 0; i < routeMigrateStatus.status.conditions.length; i++) {
          logger.verbose(`${openshiftProject}: active/standby switch status: ${routeMigrateStatus.status.conditions[i].type}`);
          switch (routeMigrateStatus.status.conditions[i].type ) {
            case 'started':
                try {
                  // update the task to started
                  var created = convertDateFormat(routeMigrateStatus.status.conditions[i].lastTransitionTime)
                  await updateTask(sourceTaskID, {
                    status: 'ACTIVE',
                    created: created,
                  });
                } catch (err) {
                    logger.error(err);
                    throw new Error();
                }
              break;
            case 'failed':
                try {
                  // update the task to failed
                  var created = convertDateFormat(routeMigrateStatus.status.conditions[i].lastTransitionTime)
                  await updateTask(sourceTaskID, {
                    status: 'FAILED',
                    completed: created,
                  });
                  var condition = new Object();
                  // send a log off with the status information
                  condition.condition = routeMigrateStatus.status.conditions[i].condition
                  condition.activeRoutes = routeMigrateStatus.spec.routes.activeRoutes
                  condition.standbyRoutes = routeMigrateStatus.spec.routes.standbyRoutes
                  var conditionStr= JSON.stringify(condition);
                  await saveTaskLog(
                    'active-standby-switch',
                    projectOpenShift.name,
                    'failed',
                    uuid,
                    conditionStr,
                  );
                } catch (err) {
                    logger.error(err);
                    throw new Error();
                }
              return breakLoop = true;
            case 'completed':
              // swap the active/standby in lagoon by updating the project
              try {
                const response = await updateProject(projectOpenShift.id, {
                  productionEnvironment: safeStandbyProductionEnvironment,
                  standbyProductionEnvironment: safeActiveProductionEnvironment,
                  productionRoutes: routeMigrateStatus.spec.routes.activeRoutes,
                  standbyRoutes: routeMigrateStatus.spec.routes.standbyRoutes,
                });
                // update the task to completed
                var created = convertDateFormat(routeMigrateStatus.status.conditions[i].lastTransitionTime)
                await updateTask(sourceTaskID, {
                  status: 'SUCCEEDED',
                  completed: created,
                });
                // send a log off with the status information
                var condition = new Object();
                condition.condition = routeMigrateStatus.status.conditions[i].condition
                condition.activeRoutes = routeMigrateStatus.spec.routes.activeRoutes
                condition.standbyRoutes = routeMigrateStatus.spec.routes.standbyRoutes
                var conditionStr= JSON.stringify(condition);
                await saveTaskLog(
                  'active-standby-switch',
                  projectOpenShift.name,
                  'succeeded',
                  uuid,
                  conditionStr,
                );
              } catch (err) {
                  logger.error(err);
                  throw new Error();
              }
              logger.verbose(`${openshiftProject}: active/standby switch updated in lagoon`);
              return breakLoop = true;
          }
        }
      } catch (err) {
        try {
          await updateTask(sourceTaskID, {
            status: 'ACTIVE',
          });
        } catch (err) {
            logger.error(err);
            throw new Error();
        }
        logger.verbose(`${openshiftProject}: active/standby switch waiting still`);
      }
      await new Promise(resolve => setTimeout(resolve, 5000)); // wait for a bit between getting the resource
    } catch (err) {
        logger.error(err);
        throw new Error();
    }
  }

  sendToLagoonLogs(
    'info',
    projectName,
    '',
    'task:misc-openshift:route:migrate',
    data,
    `*[${projectName}]* Route Migration between environments *${destinationOpenshiftProject}* started`
  );
}

const saveTaskLog = async (jobName, projectName, status, uid, log) => {
  const meta = {
    jobName,
    jobStatus: status,
    remoteId: uid
  };

  sendToLagoonLogs(
    'info',
    projectName,
    '',
    `task:misc-openshift:route:migrate:${jobName}`,
    meta,
    log
  );
};

module.exports = routeMigration;
