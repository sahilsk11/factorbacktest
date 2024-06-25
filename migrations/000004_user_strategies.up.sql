CREATE TABLE user_strategy (
  user_strategy_id uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
  user_id uuid,
  strategy_input text not null,
  factor_expression_hash text not null,
  strategy_input_hash text not null
);
