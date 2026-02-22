import { formatDistanceToNow } from 'date-fns';
import { CheckCircle, Circle, Clock, ChevronRight } from 'lucide-react';
import { Step } from '../types';

interface StepTimelineProps {
  steps: Step[];
  onSelectStep?: (step: Step) => void;
  selectedStepId?: string;
}

export default function StepTimeline({ steps, onSelectStep, selectedStepId }: StepTimelineProps) {
  return (
    <div className="flow-root">
      <ul className="-mb-8">
        {steps.map((step, stepIdx) => {
          const isLast = stepIdx === steps.length - 1;
          const isSelected = step.step_id === selectedStepId;
          const isCompleted = !!step.ended_at;

          return (
            <li key={step.step_id}>
              <div className="relative pb-8">
                {!isLast && (
                  <span
                    className="absolute top-4 left-4 -ml-px h-full w-0.5 bg-gray-200"
                    aria-hidden="true"
                  />
                )}
                <div
                  className={`relative flex space-x-3 cursor-pointer rounded-lg p-2 -ml-2 ${
                    isSelected ? 'bg-blue-50' : 'hover:bg-gray-50'
                  }`}
                  onClick={() => onSelectStep?.(step)}
                >
                  <div>
                    <span
                      className={`h-8 w-8 rounded-full flex items-center justify-center ring-8 ring-white ${
                        isCompleted
                          ? 'bg-green-500'
                          : 'bg-blue-500'
                      }`}
                    >
                      {isCompleted ? (
                        <CheckCircle className="h-5 w-5 text-white" />
                      ) : (
                        <Circle className="h-5 w-5 text-white animate-pulse" />
                      )}
                    </span>
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between">
                      <p className="text-sm font-medium text-gray-900">
                        {step.name}
                      </p>
                      {isSelected && (
                        <ChevronRight className="h-4 w-4 text-blue-500" />
                      )}
                    </div>
                    <div className="flex items-center mt-1 text-sm text-gray-500">
                      <Clock className="h-3 w-3 mr-1" />
                      {formatDistanceToNow(new Date(step.started_at), { addSuffix: true })}
                      {step.ended_at && (
                        <span className="ml-2">
                          (duration: {Math.round(
                            (new Date(step.ended_at).getTime() - new Date(step.started_at).getTime()) / 1000
                          )}s)
                        </span>
                      )}
                    </div>
                    <p className="mt-1 text-xs text-gray-400">
                      Step #{step.seq_no}
                    </p>
                  </div>
                </div>
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
