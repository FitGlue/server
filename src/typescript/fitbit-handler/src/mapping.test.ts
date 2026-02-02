
import { ActivityType } from '@fitglue/shared/dist/types/pb/standardized_activity';
import { mapFitbitActivityType } from './connector';

describe('mapFitbitActivityType', () => {
  const testCases: [string | undefined, ActivityType, string][] = [
    // Running
    ['Run', ActivityType.ACTIVITY_TYPE_RUN, 'Simple Run'],
    ['Morning Run', ActivityType.ACTIVITY_TYPE_RUN, 'Run with prefix'],
    ['Treadmill', ActivityType.ACTIVITY_TYPE_RUN, 'Treadmill'],
    ['Jogging', ActivityType.ACTIVITY_TYPE_RUN, 'Jogging'],
    ['Trail Run', ActivityType.ACTIVITY_TYPE_TRAIL_RUN, 'Trail Run'],
    ['Virtual Run', ActivityType.ACTIVITY_TYPE_VIRTUAL_RUN, 'Virtual Run'],

    // Walking
    ['Walk', ActivityType.ACTIVITY_TYPE_WALK, 'Walk'],
    ['Power Walk', ActivityType.ACTIVITY_TYPE_WALK, 'Power Walk'],

    // Cycling
    ['Bike', ActivityType.ACTIVITY_TYPE_RIDE, 'Bike'],
    ['Outdoor Bike', ActivityType.ACTIVITY_TYPE_RIDE, 'Outdoor Bike'],
    ['Cycling', ActivityType.ACTIVITY_TYPE_RIDE, 'Cycling'],
    ['Spinning', ActivityType.ACTIVITY_TYPE_RIDE, 'Spinning'],
    ['Spin', ActivityType.ACTIVITY_TYPE_RIDE, 'Spin'],

    // Swimming
    ['Swim', ActivityType.ACTIVITY_TYPE_SWIM, 'Swim'],
    ['Pool Swim', ActivityType.ACTIVITY_TYPE_SWIM, 'Pool Swim'],

    // Weight Training
    ['Weights', ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING, 'Weights'],
    ['Weight Lifting', ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING, 'Weight Lifting'],
    ['Strength Training', ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING, 'Strength Training'],
    ['Upper Body', ActivityType.ACTIVITY_TYPE_WORKOUT, 'Upper Body (Expect Failure/Workout)'], // Likely fallback

    // Other
    ['Hike', ActivityType.ACTIVITY_TYPE_HIKE, 'Hike'],
    ['Yoga', ActivityType.ACTIVITY_TYPE_YOGA, 'Yoga'],
    ['Elliptical', ActivityType.ACTIVITY_TYPE_ELLIPTICAL, 'Elliptical'],
    ['Generic Workout', ActivityType.ACTIVITY_TYPE_WORKOUT, 'Generic Workout'],

    // Edge Cases
    [undefined, ActivityType.ACTIVITY_TYPE_WORKOUT, 'Undefined'],
    ['', ActivityType.ACTIVITY_TYPE_WORKOUT, 'Empty String'],
    ['   Run   ', ActivityType.ACTIVITY_TYPE_RUN, 'Whitespace Trimming'],
    ['RUN', ActivityType.ACTIVITY_TYPE_RUN, 'Case Insensitivity'],

    // Specific User Request
    ['Structured Workout', ActivityType.ACTIVITY_TYPE_RUN, 'Structured Workout -> Run'],
    ['structured workout', ActivityType.ACTIVITY_TYPE_RUN, 'structured workout -> Run'],
    ['My Structured Workout', ActivityType.ACTIVITY_TYPE_RUN, 'Containing structured workout -> Run'],
  ];

  test.each(testCases)('maps "%s" to %s (%s)', (input, expected, _desc) => {
    expect(mapFitbitActivityType(input)).toBe(expected);
  });
});
